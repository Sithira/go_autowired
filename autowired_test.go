package autowired_test

import (
	"errors"
	"me.sithiramunasinghe/go-autowired"
	"testing"
)

// Simple service for testing
type TestService struct {
	Value string
}

func NewTestService() *TestService {
	return &TestService{Value: "default"}
}

// Test basic registration and resolution
func TestBasicRegistrationAndResolution(t *testing.T) {
	container := autowired.NewContainer()

	err := autowired.Register[TestService](container, NewTestService)
	if err != nil {
		t.Fatalf("Failed to register TestService: %v", err)
	}

	service, err := autowired.Resolve[*TestService](container)
	if err != nil {
		t.Fatalf("Failed to resolve TestService: %v", err)
	}

	if service.Value != "default" {
		t.Errorf("Expected value 'default', got '%s'", service.Value)
	}
}

// Test different scopes
func TestScopes(t *testing.T) {
	container := autowired.NewContainer()

	// Singleton scope
	err := autowired.Register[TestService](container, NewTestService)
	if err != nil {
		t.Fatalf("Failed to register singleton TestService: %v", err)
	}

	singleton1, _ := autowired.Resolve[*TestService](container)
	singleton2, _ := autowired.Resolve[*TestService](container)

	if singleton1 != singleton2 {
		t.Error("Singleton instances should be the same")
	}

	// Prototype scope
	err = autowired.Register[TestService](container, NewTestService, autowired.Prototype)
	if err != nil {
		t.Fatalf("Failed to register prototype TestService: %v", err)
	}

	prototype1, _ := autowired.Resolve[*TestService](container)
	prototype2, _ := autowired.Resolve[*TestService](container)

	if prototype1 == prototype2 {
		t.Error("Prototype instances should be different")
	}
}

// Test lifecycle hooks
func TestLifecycleHooks(t *testing.T) {
	container := autowired.NewContainer()

	initCalled := false
	startCalled := false
	destroyCalled := false

	hooks := autowired.LifecycleHooks[*TestService]{
		OnInit: func(s *TestService) error {
			initCalled = true
			return nil
		},
		OnStart: func(s *TestService) error {
			startCalled = true
			return nil
		},
		OnDestroy: func(s *TestService) error {
			destroyCalled = true
			return nil
		},
	}

	err := autowired.Register[TestService](container, NewTestService, hooks)
	if err != nil {
		t.Fatalf("Failed to register TestService with hooks: %v", err)
	}

	_, err = autowired.Resolve[*TestService](container)
	if err != nil {
		t.Fatalf("Failed to resolve TestService: %v", err)
	}

	if !initCalled || !startCalled {
		t.Error("Init and Start hooks should have been called")
	}

	err = container.Destroy()
	if err != nil {
		t.Fatalf("Failed to destroy container: %v", err)
	}

	if !destroyCalled {
		t.Error("Destroy hook should have been called")
	}
}

// Test auto-wiring
func TestAutoWire(t *testing.T) {
	container := autowired.NewContainer()

	err := autowired.Register[TestService](container, NewTestService)
	if err != nil {
		t.Fatalf("Failed to register TestService: %v", err)
	}

	type TestApp struct {
		Service *TestService `autowire:""`
	}

	app := &TestApp{}
	err = autowired.AutoWire(container, app)
	if err != nil {
		t.Fatalf("Failed to auto-wire TestApp: %v", err)
	}

	if app.Service == nil {
		t.Error("TestService should have been auto-wired")
	}
}

type ServiceB struct {
	A *ServiceA
}

type ServiceA struct {
	B *ServiceB
}

// Test circular dependency detection
func TestCircularDependency(t *testing.T) {
	container := autowired.NewContainer()

	err := autowired.Register[ServiceA](container, func(b *ServiceB) *ServiceA {
		return &ServiceA{B: b}
	})
	if err != nil {
		t.Fatalf("Failed to register ServiceA: %v", err)
	}

	err = autowired.Register[ServiceB](container, func(a *ServiceA) *ServiceB {
		return &ServiceB{A: a}
	})
	if err != nil {
		t.Fatalf("Failed to register ServiceB: %v", err)
	}

	_, err = autowired.Resolve[*ServiceA](container)
	if err == nil {
		t.Error("Expected circular dependency error, got nil")
	}
}

// Test custom naming
func TestCustomNaming(t *testing.T) {
	container := autowired.NewContainer()

	err := autowired.Register[TestService](container, NewTestService, "custom")
	if err != nil {
		t.Fatalf("Failed to register TestService with custom name: %v", err)
	}

	_, err = autowired.Resolve[*TestService](container, "custom")
	if err != nil {
		t.Fatalf("Failed to resolve TestService with custom name: %v", err)
	}

	_, err = autowired.Resolve[*TestService](container)
	if err == nil {
		t.Error("Expected error when resolving without custom name, got nil")
	}
}

// Test error handling
func TestErrorHandling(t *testing.T) {
	container := autowired.NewContainer()

	// Test registration with invalid constructor
	err := autowired.Register[TestService](container, "not a function")
	if err == nil {
		t.Error("Expected error when registering invalid constructor, got nil")
	}

	// Test resolution of unregistered dependency
	_, err = autowired.Resolve[*TestService](container)
	if err == nil {
		t.Error("Expected error when resolving unregistered dependency, got nil")
	}

	// Test constructor returning error
	err = autowired.Register[TestService](container, func() (*TestService, error) {
		return nil, errors.New("constructor error")
	})
	if err != nil {
		t.Fatalf("Failed to register TestService with error constructor: %v", err)
	}

	_, err = autowired.Resolve[*TestService](container)
	if err == nil {
		t.Error("Expected error from constructor, got nil")
	}
}
