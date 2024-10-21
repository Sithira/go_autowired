# @AutoWired for Go 🚀

<img alt="autowired_golang" height="300" src="/golang_autowired_logo.webp" title="GoLang Autowired" width="300"/>

`autowired` is a powerful and flexible dependency injection container for Go applications. It provides a robust set of features for managing dependencies, including lifecycle management, scoping, and automatic wiring of structs. This package is designed to simplify dependency management in complex Go applications while maintaining type safety and allowing for flexible configuration.

## Features

- Dependency registration with custom names and scopes
- Support for Singleton, Prototype, and Request scopes
- Lifecycle hooks (OnInit, OnStart, OnDestroy)
- Automatic dependency resolution with circular dependency detection
- Type-safe wrappers for common operations
- Struct field auto-wiring
- Thread-safe operations
- Lazy dependency resolution

## Installation

To use `autowired` in your Go project, you can install it using:

```bash
go get github.com/yourusername/autowired
```

Replace `yourusername` with the appropriate GitHub username or organization.

## Usage

### Creating a Container

```go
container := autowired.NewContainer()
```

### Registering Dependencies

You can register dependencies with different scopes:

```go
// Singleton (default)
err := autowired.Register[MyService](container, func() *MyService {
    return &MyService{}
})

// Prototype
err := autowired.Register[MyPrototypeService](container, func() *MyPrototypeService {
    return &MyPrototypeService{}
}, autowired.Prototype)

// Request
err := autowired.Register[MyRequestService](container, func() *MyRequestService {
    return &MyRequestService{}
}, autowired.Request)
```

### Resolving Dependencies

```go
service, err := autowired.Resolve[*MyService](container)
if err != nil {
    // Handle error
}
```

### Auto-wiring Structs

```go
type MyApp struct {
    Service *MyService `autowire:""`
}

app := &MyApp{}
err := autowired.AutoWire(container, app)
if err != nil {
    // Handle error
}
```

### Using Scoped Dependencies

#### Singleton Scope (Default)

Singleton-scoped dependencies are created once and reused for all resolutions:

```go
type Counter struct {
    count int
}

func (c *Counter) Increment() {
    c.count++
}

err := autowired.Register[Counter](container, func() *Counter {
    return &Counter{}
})

counter1, _ := autowired.Resolve[*Counter](container)
counter1.Increment()

counter2, _ := autowired.Resolve[*Counter](container)
counter2.Increment()

fmt.Println(counter1.count) // Output: 2
fmt.Println(counter2.count) // Output: 2 (same instance)
```

#### Prototype Scope

Prototype-scoped dependencies are created anew for each resolution:

```go
err := autowired.Register[Counter](container, func() *Counter {
    return &Counter{}
}, autowired.Prototype)

counter1, _ := autowired.Resolve[*Counter](container)
counter1.Increment()

counter2, _ := autowired.Resolve[*Counter](container)
counter2.Increment()

fmt.Println(counter1.count) // Output: 1
fmt.Println(counter2.count) // Output: 1 (different instance)
```

#### Request Scope

Request-scoped dependencies are created once per goroutine (typically per HTTP request):

```go
err := autowired.Register[RequestContext](container, func() *RequestContext {
    return &RequestContext{}
}, autowired.Request)

http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    ctx, _ := autowired.Resolve[*RequestContext](container)
    // Use ctx...
    
    defer container.ClearRequestScoped()
})
```

### Lifecycle Hooks

You can define lifecycle hooks for your dependencies:

```go
hooks := autowired.LifecycleHooks[*MyService]{
    OnInit: func(s *MyService) error {
        fmt.Println("Initializing MyService")
        return nil
    },
    OnStart: func(s *MyService) error {
        fmt.Println("Starting MyService")
        return nil
    },
    OnDestroy: func(s *MyService) error {
        fmt.Println("Destroying MyService")
        return nil
    },
}

err := autowired.Register[MyService](container, func() *MyService {
    return &MyService{}
}, hooks)
```

### Handling Circular Dependencies

The container automatically detects circular dependencies:

```go
err := autowired.Register[ServiceA](container, func(b *ServiceB) *ServiceA {
    return &ServiceA{B: b}
})

err := autowired.Register[ServiceB](container, func(a *ServiceA) *ServiceB {
    return &ServiceB{A: a}
})

// This will return an error due to circular dependency
_, err := autowired.Resolve[*ServiceA](container)
if err != nil {
    fmt.Println("Circular dependency detected:", err)
}
```

### Custom Naming

You can register dependencies with custom names:

```go
err := autowired.Register[MyService](container, func() *MyService {
    return &MyService{}
}, "customName")

service, err := autowired.Resolve[*MyService](container, "customName")
```

### Container Cleanup

Don't forget to clean up the container when you're done:

```go
err := container.Destroy()
if err != nil {
    // Handle error
}
```

## Best Practices

1. Use Singleton scope for stateless services or when you want to share state across the application.
2. Use Prototype scope when you need a fresh instance each time or for stateful services that shouldn't share state.
3. Use Request scope in web applications for request-specific data.
4. Always handle errors returned by Register, Resolve, and AutoWire.
5. Use lifecycle hooks for proper resource management.
6. Avoid circular dependencies in your application design.

## Thread Safety

The `autowired` package is designed to be thread-safe, allowing for concurrent use in multi-threaded applications. However, be mindful of the thread safety of the dependencies you're managing within the container.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.