# @AutoWired for Go

`autowired_v2` is a powerful and flexible dependency injection container for Go applications. It provides a robust set of features for managing dependencies, including lifecycle management, scoping, and automatic wiring of structs.

## Features

- Dependency registration with custom names and scopes
- Singleton, Prototype, and Request scopes
- Lifecycle hooks (OnInit, OnStart, OnDestroy)
- Automatic dependency resolution
- Type-safe wrappers for common operations
- Struct field auto-wiring
- Thread-safe operations

## Installation

To use `autowired_v2` in your Go project, you can install it using:

```
go get github.com/yourusername/autowired_v2
```

Replace `yourusername` with the appropriate GitHub username or organization.

## Usage

### Creating a Container

```go
container := autowired_v2.NewContainer()
```

### Registering Dependencies

```go
type MyService struct {}

err := autowired_v2.Register[MyService](container, func() *MyService {
    return &MyService{}
})
```

### Resolving Dependencies

```go
service, err := autowired_v2.Resolve[*MyService](container)
```

### Auto-wiring Structs

```go
type MyApp struct {
    Service *MyService `autowire:""`
}

app := &MyApp{}
err := autowired_v2.AutoWire(container, app)
```

### Using Prototype-Scoped Dependencies

Prototype-scoped dependencies are useful when you want a new instance of a dependency every time it's resolved, regardless of where or how often it's requested. Here's how to use them:

1. Register a prototype-scoped dependency:

```go
err := autowired_v2.Register[MyPrototypeService](container, func() *MyPrototypeService {
    return &MyPrototypeService{}
}, autowired_v2.Prototype)
```

2. Resolve the dependency:

```go
service1, err := autowired_v2.Resolve[*MyPrototypeService](container)
if err != nil {
    // Handle error
}

service2, err := autowired_v2.Resolve[*MyPrototypeService](container)
if err != nil {
    // Handle error
}

// service1 and service2 are different instances
```

Key points about Prototype-scoped dependencies:

- A new instance is created every time the dependency is resolved.
- There's no shared state between different instances of the same prototype-scoped dependency.
- Prototype scope is useful for stateful services where you don't want to share state between different parts of your application.
- Unlike Singleton-scoped dependencies, you don't need to worry about concurrent access to prototype-scoped dependencies, as each resolution gets its own instance.
- Unlike Request-scoped dependencies, prototype-scoped dependencies don't need to be cleared after use.

Example use case:

```go
type Counter struct {
    count int
}

func (c *Counter) Increment() {
    c.count++
}

func (c *Counter) GetCount() int {
    return c.count
}

// Register the Counter as a prototype-scoped dependency
err := autowired_v2.Register[Counter](container, func() *Counter {
    return &Counter{}
}, autowired_v2.Prototype)

// In different parts of your application:
counter1, _ := autowired_v2.Resolve[*Counter](container)
counter1.Increment()
counter1.Increment()
fmt.Println(counter1.GetCount()) // Output: 2

counter2, _ := autowired_v2.Resolve[*Counter](container)
counter2.Increment()
fmt.Println(counter2.GetCount()) // Output: 1
```

In this example, `counter1` and `counter2` are separate instances, each with its own state.

Prototype scope is particularly useful when:
- You need a fresh instance of a dependency each time it's used
- The dependency has mutable state that shouldn't be shared
- You want to avoid potential concurrency issues that might arise with shared instances

Remember, while prototype-scoped dependencies provide a new instance each time, they can lead to increased memory usage if overused. Consider the trade-offs between Singleton, Request, and Prototype scopes based on your specific use case.

### Using Request-Scoped Dependencies

Request-scoped dependencies are particularly useful in web applications where you want to create a new instance of a dependency for each incoming request. Here's how to use them:

1. Register a request-scoped dependency:

```go
err := autowired_v2.Register[MyRequestScopedService](container, func() *MyRequestScopedService {
    return &MyRequestScopedService{}
}, autowired_v2.Request)
```

2. Resolve the dependency within a request handler:

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    service, err := autowired_v2.Resolve[*MyRequestScopedService](container)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Use the service...
}
```

3. Clear request-scoped dependencies after handling the request:

```go
func middleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        defer container.ClearRequestScoped()
        next(w, r)
    }
}
```

4. Use the middleware in your HTTP server setup:

```go
http.HandleFunc("/", middleware(handleRequest))
```

When using request-scoped dependencies:

- A new instance is created for each goroutine (typically corresponding to a single request in web applications).
- The same instance is returned for subsequent resolutions within the same goroutine.
- You must call `container.ClearRequestScoped()` after each request to clean up the instances and prevent memory leaks.

Remember that request-scoped dependencies are tied to the lifetime of a single request or goroutine. They are useful for maintaining state that should not be shared between different requests, such as user-specific information or request-specific caches.

### Lifecycle Hooks

```go
hooks := autowired_v2.LifecycleHooks[*MyService]{
    OnInit: func(s *MyService) error {
        // Initialization logic
        return nil
    },
    OnStart: func(s *MyService) error {
        // Start-up logic
        return nil
    },
    OnDestroy: func(s *MyService) error {
        // Clean-up logic
        return nil
    },
}

err := autowired_v2.Register[MyService](container, func() *MyService {
    return &MyService{}
}, hooks)
```



## Advanced Features

- Custom naming for dependencies
- Request-scoped dependencies
- Clearing request-scoped dependencies
- Container destruction and resource cleanup

## Thread Safety

The `autowired_v2` package is designed to be thread-safe, allowing for concurrent use in multi-threaded applications.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.