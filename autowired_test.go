package autowired

import (
	"context"
	"testing"
)

// TestNewContainer tests the creation of a new container
func TestNewContainer(t *testing.T) {
	container := NewContainer()
	if container == nil {
		t.Error("Expected non-nil container")
	}
}

// TestRegisterAndResolve tests registering and resolving a simple dependency
func TestRegisterAndResolve(t *testing.T) {
	container := NewContainer()

	type TestService struct {
		Name string
	}

	container.Register((*TestService)(nil), Singleton, func() *TestService {
		return &TestService{Name: "Test"}
	})

	ctx := context.Background()
	resolved, err := container.Resolve(ctx, (*TestService)(nil))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	service, ok := resolved.(*TestService)
	if !ok {
		t.Error("Resolved instance is not of type TestService")
	}

	if service.Name != "Test" {
		t.Errorf("Expected Name to be 'Test', got '%s'", service.Name)
	}
}

// TestRegisterNamedAndResolveNamed tests registering and resolving a named dependency
func TestRegisterNamedAndResolveNamed(t *testing.T) {
	container := NewContainer()

	type TestService struct {
		Name string
	}

	container.RegisterNamed((*TestService)(nil), "service1", Singleton, func() *TestService {
		return &TestService{Name: "Service1"}
	})

	container.RegisterNamed((*TestService)(nil), "service2", Singleton, func() *TestService {
		return &TestService{Name: "Service2"}
	})

	ctx := context.Background()

	resolved1, err := container.ResolveNamed(ctx, (*TestService)(nil), "service1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	service1, ok := resolved1.(*TestService)
	if !ok {
		t.Error("Resolved instance is not of type TestService")
	}

	if service1.Name != "Service1" {
		t.Errorf("Expected Name to be 'Service1', got '%s'", service1.Name)
	}

	resolved2, err := container.ResolveNamed(ctx, (*TestService)(nil), "service2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	service2, ok := resolved2.(*TestService)
	if !ok {
		t.Error("Resolved instance is not of type TestService")
	}

	if service2.Name != "Service2" {
		t.Errorf("Expected Name to be 'Service2', got '%s'", service2.Name)
	}
}

// TestRegisterWithHooks tests registering a dependency with hooks
func TestRegisterWithHooks(t *testing.T) {
	container := NewContainer()

	type TestService struct {
		Name  string
		State string
	}

	initCalled := false
	startCalled := false
	stopCalled := false

	container.RegisterWithHooks((*TestService)(nil), Singleton, func() *TestService {
		return &TestService{Name: "Test"}
	}, Hooks{
		Init: func(instance interface{}) error {
			initCalled = true
			instance.(*TestService).State = "Initialized"
			return nil
		},
		Start: func(instance interface{}) error {
			startCalled = true
			instance.(*TestService).State = "Started"
			return nil
		},
		Stop: func(instance interface{}) error {
			stopCalled = true
			instance.(*TestService).State = "Stopped"
			return nil
		},
	})

	ctx := context.Background()

	// Resolve should trigger Init
	resolved, err := container.Resolve(ctx, (*TestService)(nil))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	service := resolved.(*TestService)
	if !initCalled {
		t.Error("Init hook was not called")
	}
	if service.State != "Initialized" {
		t.Errorf("Expected State to be 'Initialized', got '%s'", service.State)
	}

	// Start should trigger Start hook
	err = container.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !startCalled {
		t.Error("Start hook was not called")
	}
	if service.State != "Started" {
		t.Errorf("Expected State to be 'Started', got '%s'", service.State)
	}

	// Stop should trigger Stop hook
	container.Stop()

	if !stopCalled {
		t.Error("Stop hook was not called")
	}
	if service.State != "Stopped" {
		t.Errorf("Expected State to be 'Stopped', got '%s'", service.State)
	}
}

// TestScope tests scoped dependencies
func TestScope(t *testing.T) {
	container := NewContainer()

	type TestService struct {
		ID int
	}

	idCounter := 0
	container.Register((*TestService)(nil), Scoped, func() *TestService {
		idCounter++
		return &TestService{ID: idCounter}
	})

	ctx := context.Background()

	// Create two scopes
	scope1 := container.CreateScope(ctx)
	scope2 := container.CreateScope(ctx)

	// Resolve in scope1
	resolved1, err := container.Resolve(scope1, (*TestService)(nil))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	service1 := resolved1.(*TestService)

	// Resolve again in scope1
	resolved1Again, err := container.Resolve(scope1, (*TestService)(nil))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	service1Again := resolved1Again.(*TestService)

	// Resolve in scope2
	resolved2, err := container.Resolve(scope2, (*TestService)(nil))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	service2 := resolved2.(*TestService)

	// Check that the same instance is returned within a scope
	if service1.ID != service1Again.ID {
		t.Errorf("Expected same instance within scope, got IDs %d and %d", service1.ID, service1Again.ID)
	}

	// Check that different instances are returned in different scopes
	if service1.ID == service2.ID {
		t.Errorf("Expected different instances in different scopes, got same ID %d", service1.ID)
	}
}
