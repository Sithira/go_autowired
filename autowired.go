package autowired

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"unicode"
)

// Scope represents the lifecycle of a dependency
type Scope int

const (
	Singleton Scope = iota
	Prototype
	Request
)

// Container represents the dependency injection container
type Container struct {
	dependencies map[reflect.Type]map[string]*dependencyInfo
	mu           sync.RWMutex
	resolving    sync.Map
}

// dependencyInfo holds information about a registered dependency
type dependencyInfo struct {
	constructor  reflect.Value
	scope        Scope
	instance     atomic.Value
	initOnce     sync.Once
	hooks        interface{}
	instancePool sync.Map
}

// LifecycleHooks defines lifecycle hooks for dependencies
type LifecycleHooks[T any] struct {
	OnInit    func(T) error
	OnStart   func(T) error
	OnDestroy func(T) error
}

// NewContainer creates a new Container
func NewContainer() *Container {
	return &Container{
		dependencies: make(map[reflect.Type]map[string]*dependencyInfo),
	}
}

// Register registers a dependency in the container
func (c *Container) Register(constructor interface{}, options ...interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	constructorType := reflect.TypeOf(constructor)
	if constructorType.Kind() != reflect.Func {
		return fmt.Errorf("constructor must be a function")
	}

	if constructorType.NumOut() == 0 || (constructorType.NumOut() == 2 && !constructorType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem())) {
		return fmt.Errorf("constructor must return (T) or (T, error)")
	}

	typ := constructorType.Out(0)
	name, scope, hooks := c.processOptions(typ, options...)

	if _, exists := c.dependencies[typ]; !exists {
		c.dependencies[typ] = make(map[string]*dependencyInfo)
	}

	c.dependencies[typ][name] = &dependencyInfo{
		constructor:  reflect.ValueOf(constructor),
		scope:        scope,
		hooks:        hooks,
		instancePool: sync.Map{},
	}

	return nil
}

// Resolve resolves a dependency from the container
func (c *Container) Resolve(typ reflect.Type, options ...interface{}) (interface{}, error) {
	name := c.getResolveName(options...)

	// Check for circular dependencies
	if _, resolving := c.resolving.LoadOrStore(typ, true); resolving {
		return nil, fmt.Errorf("circular dependency detected for type %v", typ)
	}
	defer c.resolving.Delete(typ)

	c.mu.RLock()
	info, err := c.getDependencyInfo(typ, name)
	c.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	return c.resolveDependency(info)
}

func (c *Container) processOptions(typ reflect.Type, options ...interface{}) (string, Scope, interface{}) {
	var name string
	scope := Singleton
	var hooks interface{}

	for _, option := range options {
		switch v := option.(type) {
		case string:
			name = v
		case Scope:
			scope = v
		default:
			if h, ok := isLifecycleHooks(v); ok {
				hooks = h
			}
		}
	}

	if name == "" {
		name = getDefaultName(typ)
	}

	return name, scope, hooks
}

func (c *Container) getResolveName(options ...interface{}) string {
	for _, option := range options {
		if n, ok := option.(string); ok {
			return n
		}
	}
	return ""
}

func (c *Container) getDependencyInfo(typ reflect.Type, name string) (*dependencyInfo, error) {
	implementations, exists := c.dependencies[typ]
	if !exists {
		return nil, fmt.Errorf("no dependency registered for type %v", typ)
	}

	if name == "" {
		name = getDefaultName(typ)
	}

	info, exists := implementations[name]
	if !exists {
		return nil, fmt.Errorf("no dependency named '%s' registered for type %v", name, typ)
	}

	return info, nil
}

func (c *Container) resolveDependency(info *dependencyInfo) (interface{}, error) {
	switch info.scope {
	case Singleton:
		return c.resolveSingleton(info)
	case Prototype:
		return c.construct(info)
	case Request:
		return c.resolveRequest(info)
	default:
		return nil, fmt.Errorf("unknown scope: %v", info.scope)
	}
}

func (c *Container) resolveSingleton(info *dependencyInfo) (interface{}, error) {
	var err error
	info.initOnce.Do(func() {
		var instance interface{}
		instance, err = c.construct(info)
		if err == nil {
			info.instance.Store(instance)
		}
	})

	if err != nil {
		return nil, err
	}

	return info.instance.Load(), nil
}

func (c *Container) resolveRequest(info *dependencyInfo) (interface{}, error) {
	key := getGoroutineID()
	if instance, ok := info.instancePool.Load(key); ok {
		return instance, nil
	}

	instance, err := c.construct(info)
	if err != nil {
		return nil, err
	}

	info.instancePool.Store(key, instance)
	return instance, nil
}

func (c *Container) construct(info *dependencyInfo) (interface{}, error) {
	params, err := c.resolveConstructorParams(info.constructor.Type())
	if err != nil {
		return nil, err
	}

	results := info.constructor.Call(params)
	if len(results) == 2 && !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	instance := results[0].Interface()

	if hooks, ok := info.hooks.(LifecycleHooks[interface{}]); ok {
		if hooks.OnInit != nil {
			if err := hooks.OnInit(instance); err != nil {
				return nil, err
			}
		}
		if hooks.OnStart != nil {
			if err := hooks.OnStart(instance); err != nil {
				return nil, err
			}
		}
	}

	return instance, nil
}

func (c *Container) resolveConstructorParams(constructorType reflect.Type) ([]reflect.Value, error) {
	params := make([]reflect.Value, constructorType.NumIn())
	for i := 0; i < constructorType.NumIn(); i++ {
		paramType := constructorType.In(i)
		param, err := c.Resolve(paramType)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter %d of type %v: %w", i, paramType, err)
		}
		params[i] = reflect.ValueOf(param)
	}
	return params, nil
}

// AutoWire automatically injects dependencies into the fields of the given struct
func (c *Container) AutoWire(target interface{}) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		tag := t.Field(i).Tag.Get("autowire")

		if tag == "-" {
			continue
		}

		var options []interface{}
		if tag != "" {
			options = append(options, tag)
		}

		dependency, err := c.Resolve(field.Type(), options...)
		if err != nil {
			return fmt.Errorf("failed to autowire field %s: %w", t.Field(i).Name, err)
		}

		field.Set(reflect.ValueOf(dependency))
	}

	return nil
}

func (c *Container) Destroy() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, implementations := range c.dependencies {
		for _, info := range implementations {
			if hooks, ok := info.hooks.(LifecycleHooks[interface{}]); ok {
				if hooks.OnDestroy != nil {
					instance := info.instance.Load()
					if instance != nil {
						if err := hooks.OnDestroy(instance); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// ClearRequestScoped clears all request-scoped dependencies
func (c *Container) ClearRequestScoped() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, implementations := range c.dependencies {
		for _, info := range implementations {
			if info.scope == Request {
				info.instancePool = sync.Map{}
			}
		}
	}
}

// Helper functions

func toCamelCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func getDefaultName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return toCamelCase(t.Name())
}

func getGoroutineID() uint64 {
	return uint64(reflect.ValueOf(make(chan int)).Pointer())
}

func isLifecycleHooks(v interface{}) (LifecycleHooks[interface{}], bool) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		return LifecycleHooks[interface{}]{}, false
	}

	rt := rv.Type()
	if rt.NumField() != 3 {
		return LifecycleHooks[interface{}]{}, false
	}

	onInitField, hasOnInit := rt.FieldByName("OnInit")
	onStartField, hasOnStart := rt.FieldByName("OnStart")
	onDestroyField, hasOnDestroy := rt.FieldByName("OnDestroy")

	if !hasOnInit || !hasOnStart || !hasOnDestroy {
		return LifecycleHooks[interface{}]{}, false
	}

	isValidHook := func(f reflect.StructField) bool {
		return f.Type.Kind() == reflect.Func &&
			f.Type.NumIn() == 1 &&
			f.Type.NumOut() == 1 &&
			f.Type.Out(0) == reflect.TypeOf((*error)(nil)).Elem()
	}

	if !isValidHook(onInitField) || !isValidHook(onStartField) || !isValidHook(onDestroyField) {
		return LifecycleHooks[interface{}]{}, false
	}

	return LifecycleHooks[interface{}]{
		OnInit:    convertToInterfaceFunc(rv.FieldByName("OnInit")),
		OnStart:   convertToInterfaceFunc(rv.FieldByName("OnStart")),
		OnDestroy: convertToInterfaceFunc(rv.FieldByName("OnDestroy")),
	}, true
}

func convertToInterfaceFunc(v reflect.Value) func(interface{}) error {
	if v.IsNil() {
		return nil
	}
	return func(i interface{}) error {
		results := v.Call([]reflect.Value{reflect.ValueOf(i)})
		if len(results) == 0 {
			return nil
		}
		err, _ := results[0].Interface().(error)
		return err
	}
}

// Type-safe wrappers

func Register[T any](c *Container, constructor interface{}, options ...interface{}) error {
	return c.Register(constructor, options...)
}

func Resolve[T any](c *Container, options ...interface{}) (T, error) {
	var t T
	instance, err := c.Resolve(reflect.TypeOf(&t).Elem(), options...)
	if err != nil {
		return t, err
	}
	return instance.(T), nil
}

func AutoWire[T any](c *Container, target *T) error {
	return c.AutoWire(target)
}
