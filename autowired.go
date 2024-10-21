package autowired

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// DefaultName Constants consts
const (
	DefaultName = ""
)

// Lifetime represents the lifetime of a dependency
type Lifetime int

const (
	Singleton Lifetime = iota
	Scoped
	Transient
)

// Hook represents a function that can be called on a dependency instance
type Hook func(instance interface{}) error

// Hooks represents the different hooks that can be attached to a dependency
type Hooks struct {
	Init  Hook
	Start Hook
	Stop  Hook
}

// Factory is a function that creates an instance of a dependency
type Factory func(ctx context.Context, c *Container) (interface{}, error)

// Registration represents a dependency registration
type Registration struct {
	lifetime    Lifetime
	constructor interface{}
	factory     Factory
	hooks       Hooks
	name        string
}

// dependencyNode represents a node in the dependency graph
type dependencyNode struct {
	t    reflect.Type
	name string
}

// String returns a string representation of a dependencyNode
func (n dependencyNode) String() string {
	if n.name == DefaultName {
		return n.t.String()
	}
	return fmt.Sprintf("%s (name: %s)", n.t.String(), n.name)
}

// Scope represents a dependency injection scope
type Scope struct {
	instances   map[dependencyNode]interface{}
	startedFlag map[dependencyNode]bool
	mu          sync.RWMutex
}

// Container represents the dependency injection container
type Container struct {
	registrations map[reflect.Type]map[string]Registration
	singletons    map[reflect.Type]map[string]interface{}
	startedFlag   map[dependencyNode]bool
	mu            sync.RWMutex
	graph         map[dependencyNode][]dependencyNode
}

// NewContainer creates a new dependency injection container
func NewContainer() *Container {
	return &Container{
		registrations: make(map[reflect.Type]map[string]Registration),
		singletons:    make(map[reflect.Type]map[string]interface{}),
		startedFlag:   make(map[dependencyNode]bool),
		graph:         make(map[dependencyNode][]dependencyNode),
	}
}

// Register registers a dependency with the container
func (c *Container) Register(iface interface{}, lifetime Lifetime, constructor interface{}) {
	c.RegisterNamed(iface, DefaultName, lifetime, constructor)
}

// RegisterNamed registers a named dependency with the container
func (c *Container) RegisterNamed(iface interface{}, name string, lifetime Lifetime, constructor interface{}) {
	c.RegisterNamedWithHooks(iface, name, lifetime, constructor, Hooks{})
}

// RegisterWithHooks registers a dependency with hooks
func (c *Container) RegisterWithHooks(iface interface{}, lifetime Lifetime, constructor interface{}, hooks Hooks) {
	c.RegisterNamedWithHooks(iface, DefaultName, lifetime, constructor, hooks)
}

// RegisterNamedWithHooks registers a named dependency with hooks
func (c *Container) RegisterNamedWithHooks(iface interface{}, name string, lifetime Lifetime, constructor interface{}, hooks Hooks) {
	c.mu.Lock()
	defer c.mu.Unlock()

	t := reflect.TypeOf(iface).Elem()
	if c.registrations[t] == nil {
		c.registrations[t] = make(map[string]Registration)
	}
	c.registrations[t][name] = Registration{
		lifetime:    lifetime,
		constructor: constructor,
		hooks:       hooks,
		name:        name,
	}

	c.updateDependencyGraph(t, name, constructor)
}

// RegisterFactory registers a factory for creating a dependency
func (c *Container) RegisterFactory(iface interface{}, lifetime Lifetime, factory Factory) {
	c.RegisterNamedFactory(iface, DefaultName, lifetime, factory)
}

// RegisterNamedFactory registers a named factory for creating a dependency
func (c *Container) RegisterNamedFactory(iface interface{}, name string, lifetime Lifetime, factory Factory) {
	c.mu.Lock()
	defer c.mu.Unlock()

	t := reflect.TypeOf(iface).Elem()
	if c.registrations[t] == nil {
		c.registrations[t] = make(map[string]Registration)
	}
	c.registrations[t][name] = Registration{
		lifetime: lifetime,
		factory:  factory,
		name:     name,
	}

	c.updateDependencyGraph(t, name, factory)
}

// updateDependencyGraph updates the dependency graph
func (c *Container) updateDependencyGraph(t reflect.Type, name string, factoryOrConstructor interface{}) {
	node := dependencyNode{t: t, name: name}

	if _, ok := factoryOrConstructor.(Factory); ok {
		// For factories, we can't determine dependencies statically
		c.graph[node] = []dependencyNode{}
	} else {
		constructorType := reflect.TypeOf(factoryOrConstructor)
		for i := 0; i < constructorType.NumIn(); i++ {
			paramType := constructorType.In(i)
			if paramType != reflect.TypeOf((*context.Context)(nil)).Elem() {
				dependencyNode := dependencyNode{t: paramType, name: DefaultName}
				c.graph[node] = append(c.graph[node], dependencyNode)
			}
		}
	}
}

// Resolve resolves a dependency
func (c *Container) Resolve(ctx context.Context, iface interface{}) (interface{}, error) {
	return c.ResolveNamed(ctx, iface, DefaultName)
}

// ResolveNamed resolves a named dependency
func (c *Container) ResolveNamed(ctx context.Context, iface interface{}, name string) (interface{}, error) {
	t := reflect.TypeOf(iface).Elem()
	node := dependencyNode{t: t, name: name}

	resolved := make(map[dependencyNode]interface{})
	if err := c.resolveDependencies(ctx, node, resolved, nil); err != nil {
		return nil, err
	}

	return resolved[node], nil
}

// resolveDependencies resolves all dependencies for a given node
func (c *Container) resolveDependencies(ctx context.Context, node dependencyNode, resolved map[dependencyNode]interface{}, stack []dependencyNode) error {
	// Check if already resolved
	if _, ok := resolved[node]; ok {
		return nil
	}

	// Check for circular dependencies
	for _, n := range stack {
		if n == node {
			return fmt.Errorf("circular dependency detected: %v", stack)
		}
	}

	c.mu.RLock()
	reg, ok := c.registrations[node.t][node.name]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no registration found for %v with name %s", node.t, node.name)
	}

	// Check if it's a singleton and already instantiated
	if reg.lifetime == Singleton {
		c.mu.RLock()
		if instance, ok := c.singletons[node.t][node.name]; ok {
			resolved[node] = instance
			c.mu.RUnlock()
			return nil
		}
		c.mu.RUnlock()
	}

	// Check if it's a scoped instance and already instantiated
	if reg.lifetime == Scoped {
		scope := c.getScope(ctx)
		if scope != nil {
			scope.mu.RLock()
			if instance, ok := scope.instances[node]; ok {
				resolved[node] = instance
				scope.mu.RUnlock()
				return nil
			}
			scope.mu.RUnlock()
		}
	}

	var instance interface{}
	var err error

	if reg.factory != nil {
		instance, err = reg.factory(ctx, c)
		if err != nil {
			return fmt.Errorf("factory for %v returned an error: %v", node.t, err)
		}
	} else if reg.constructor != nil {
		// Resolve dependencies
		constructorType := reflect.TypeOf(reg.constructor)
		params := make([]reflect.Value, constructorType.NumIn())

		for i := 0; i < constructorType.NumIn(); i++ {
			paramType := constructorType.In(i)
			if paramType == reflect.TypeOf((*context.Context)(nil)).Elem() {
				params[i] = reflect.ValueOf(ctx)
			} else {
				dependencyNode := dependencyNode{t: paramType, name: DefaultName}
				if err := c.resolveDependencies(ctx, dependencyNode, resolved, append(stack, node)); err != nil {
					return err
				}
				params[i] = reflect.ValueOf(resolved[dependencyNode])
			}
		}

		// Call constructor
		constructorValue := reflect.ValueOf(reg.constructor)
		results := constructorValue.Call(params)

		if len(results) != 1 && len(results) != 2 {
			return fmt.Errorf("constructor must return (instance) or (instance, error)")
		}

		instance = results[0].Interface()

		if len(results) == 2 {
			err, ok := results[1].Interface().(error)
			if ok && err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("no factory or constructor registered for %v", node.t)
	}

	// Call init hook if it exists
	if reg.hooks.Init != nil {
		if err := reg.hooks.Init(instance); err != nil {
			return fmt.Errorf("init hook failed for %v: %v", node.t, err)
		}
	}

	// Store the resolved instance
	switch reg.lifetime {
	case Singleton:
		c.mu.Lock()
		if c.singletons[node.t] == nil {
			c.singletons[node.t] = make(map[string]interface{})
		}
		c.singletons[node.t][node.name] = instance
		c.mu.Unlock()
	case Scoped:
		scope := c.getScope(ctx)
		if scope != nil {
			scope.mu.Lock()
			scope.instances[node] = instance
			scope.mu.Unlock()
		}
	}

	resolved[node] = instance
	return nil
}

// CreateScope creates a new dependency injection scope
func (c *Container) CreateScope(ctx context.Context) context.Context {
	scope := &Scope{
		instances:   make(map[dependencyNode]interface{}),
		startedFlag: make(map[dependencyNode]bool),
	}
	return context.WithValue(ctx, scopeKey{}, scope)
}

// getScope retrieves the current scope from the context
func (c *Container) getScope(ctx context.Context) *Scope {
	if scope, ok := ctx.Value(scopeKey{}).(*Scope); ok {
		return scope
	}
	return nil
}

// Start starts all registered dependencies
func (c *Container) Start(ctx context.Context) error {
	for t, namedRegs := range c.registrations {
		for name, reg := range namedRegs {
			if reg.hooks.Start != nil {
				node := dependencyNode{t: t, name: name}
				var instance interface{}
				var err error

				switch reg.lifetime {
				case Singleton:
					c.mu.RLock()
					instance = c.singletons[t][name]
					c.mu.RUnlock()
				case Scoped, Transient:
					instance, err = c.ResolveNamed(ctx, reflect.New(t).Interface(), name)
				}

				if err != nil {
					return fmt.Errorf("failed to resolve %v for start hook: %v", t, err)
				}

				if err := reg.hooks.Start(instance); err != nil {
					return fmt.Errorf("start hook failed for %v: %v", t, err)
				}

				if reg.lifetime == Singleton {
					c.mu.Lock()
					c.startedFlag[node] = true
					c.mu.Unlock()
				} else if reg.lifetime == Scoped {
					scope := c.getScope(ctx)
					if scope != nil {
						scope.mu.Lock()
						scope.startedFlag[node] = true
						scope.mu.Unlock()
					}
				}
			}
		}
	}
	return nil
}

// Stop stops all registered dependencies
func (c *Container) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for t, namedInstances := range c.singletons {
		for name, instance := range namedInstances {
			if reg, ok := c.registrations[t][name]; ok && reg.hooks.Stop != nil {
				node := dependencyNode{t: t, name: name}
				if started, ok := c.startedFlag[node]; ok && started {
					reg.hooks.Stop(instance)
					delete(c.startedFlag, node)
				}
			}
		}
	}
}

// DestroyScope destroys the current scope
func (c *Container) DestroyScope(ctx context.Context) {
	scope := c.getScope(ctx)
	if scope == nil {
		return
	}

	scope.mu.Lock()
	defer scope.mu.Unlock()

	for node, instance := range scope.instances {
		if reg, ok := c.registrations[node.t][node.name]; ok && reg.hooks.Stop != nil {
			if started, ok := scope.startedFlag[node]; ok && started {
				reg.hooks.Stop(instance)
			}
		}
	}

	// Clear the scope
	scope.instances = make(map[dependencyNode]interface{})
	scope.startedFlag = make(map[dependencyNode]bool)
}

// ResolveSingletonOrTransient resolves a singleton or transient dependency
func (c *Container) ResolveSingletonOrTransient(iface interface{}) (interface{}, error) {
	return c.ResolveNamedSingletonOrTransient(iface, DefaultName)
}

// ResolveNamedSingletonOrTransient resolves a named singleton or transient dependency
func (c *Container) ResolveNamedSingletonOrTransient(iface interface{}, name string) (interface{}, error) {
	ctx := context.Background()
	return c.ResolveNamed(ctx, iface, name)
}

// PrintDependencyTree prints the dependency tree
func (c *Container) PrintDependencyTree() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result strings.Builder
	visited := make(map[dependencyNode]bool)

	var printNode func(node dependencyNode, indent string)
	printNode = func(node dependencyNode, indent string) {
		if visited[node] {
			fmt.Fprintf(&result, "%s%v (circular reference)\n", indent, node)
			return
		}
		visited[node] = true

		fmt.Fprintf(&result, "%s%v\n", indent, node)
		for _, dep := range c.graph[node] {
			printNode(dep, indent+"  ")
		}
		delete(visited, node)
	}

	for node := range c.graph {
		if !visited[node] {
			printNode(node, "")
		}
	}

	return result.String()
}

// Type-safe wrapper functions

// RegisterSingleton registers a singleton dependency
func RegisterSingleton[T any](c *Container, constructor interface{}) {
	c.Register((*T)(nil), Singleton, constructor)
}

// RegisterScoped registers a scoped dependency
func RegisterScoped[T any](c *Container, constructor interface{}) {
	c.Register((*T)(nil), Scoped, constructor)
}

// RegisterTransient registers a transient dependency
func RegisterTransient[T any](c *Container, constructor interface{}) {
	c.Register((*T)(nil), Transient, constructor)
}

// RegisterSingletonWithHooks registers a singleton dependency with hooks
func RegisterSingletonWithHooks[T any](c *Container, constructor interface{}, hooks Hooks) {
	c.RegisterWithHooks((*T)(nil), Singleton, constructor, hooks)
}

// RegisterScopedWithHooks registers a scoped dependency with hooks
func RegisterScopedWithHooks[T any](c *Container, constructor interface{}, hooks Hooks) {
	c.RegisterWithHooks((*T)(nil), Scoped, constructor, hooks)
}

// RegisterTransientWithHooks registers a transient dependency with hooks
func RegisterTransientWithHooks[T any](c *Container, constructor interface{}, hooks Hooks) {
	c.RegisterWithHooks((*T)(nil), Transient, constructor, hooks)
}

// RegisterSingletonFactory registers a singleton factory
func RegisterSingletonFactory[T any](c *Container, factory Factory) {
	c.RegisterFactory((*T)(nil), Singleton, factory)
}

// RegisterScopedFactory registers a scoped factory
func RegisterScopedFactory[T any](c *Container, factory Factory) {
	c.RegisterFactory((*T)(nil), Scoped, factory)
}

// RegisterTransientFactory registers a transient factory
func RegisterTransientFactory[T any](c *Container, factory Factory) {
	c.RegisterFactory((*T)(nil), Transient, factory)
}

// Resolve resolves a dependency
func Resolve[T any](ctx context.Context, c *Container) (T, error) {
	instance, err := c.Resolve(ctx, (*T)(nil))
	if err != nil {
		var zero T
		return zero, err
	}
	return instance.(T), nil
}

// ResolveNamed resolves a named dependency
func ResolveNamed[T any](ctx context.Context, c *Container, name string) (T, error) {
	instance, err := c.ResolveNamed(ctx, (*T)(nil), name)
	if err != nil {
		var zero T
		return zero, err
	}
	return instance.(T), nil
}

// ResolveSingletonOrTransient resolves a singleton or transient dependency
func ResolveSingletonOrTransient[T any](c *Container) (T, error) {
	instance, err := c.ResolveSingletonOrTransient((*T)(nil))
	if err != nil {
		var zero T
		return zero, err
	}
	return instance.(T), nil
}

// ResolveNamedSingletonOrTransient resolves a named singleton or transient dependency
func ResolveNamedSingletonOrTransient[T any](c *Container, name string) (T, error) {
	instance, err := c.ResolveNamedSingletonOrTransient((*T)(nil), name)
	if err != nil {
		var zero T
		return zero, err
	}
	return instance.(T), nil
}

// scopeKey is used as a key for the context
type scopeKey struct{}
