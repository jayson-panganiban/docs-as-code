# Building a Docs-as-Code API: Part 2 - Design Patterns & Architectural Decisions

## Overview

In Part 1, we built the scaffolding. Now let's dive deep into why our architectural choices matter, and how they enable us to build maintainable, testable, scalable services.

This post is about the *why* behind clean architecture, dependency injection, and interface-driven design in Go.

## The Problem We Solved

Before we wrote code, we asked ourselves:

> "In 6 months, we'll need to replace the in-memory store with PostgreSQL. In a year, we might need to switch PDF converters. How do we make these changes without rewriting the application?"

The answer: **interfaces and dependency injection**.

## Pattern 1: Interface Segregation

We didn't create one monolithic "Service" interface. We split concerns:

```go
// internal/store/store.go
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
}

// internal/converter/converter.go
type Converter interface {
    MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}
```

**Why separate interfaces?**

1. **Single Responsibility**: Each interface has one reason to change
2. **Flexible Mocking**: Test handlers without initializing a converter
3. **Independent Implementation**: Build MemoryStore while waiting for converter code
4. **Substitutability**: Swap either component independently

### The Monolithic Anti-Pattern

Here's what we *avoided*:

```go
// ❌ BAD: Monolithic interface
type Service interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
    MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
    ValidateMarkdown(ctx context.Context, data []byte) error
    SendNotification(ctx context.Context, jobID string, status string) error
}
```

Problems:
- If we need to swap only the PDF converter, we reimplement everything
- Mock object must implement every method, even unused ones
- A component that only needs storage has a dependency on email systems

## Pattern 2: Dependency Injection

Our server receives its dependencies:

```go
type Server struct {
    store     store.DocumentStore
    converter converter.Converter
}

func NewServer(store store.DocumentStore, converter converter.Converter) *Server {
    return &Server{
        store:     store,
        converter: converter,
    }
}
```

**Why not create dependencies inside?**

```go
// ❌ BAD: Hard-coded dependencies
type Server struct {}

func NewServer() *Server {
    return &Server{
        store:     postgres.NewStore(os.Getenv("DB_URL")),
        converter: chromedp.NewConverter(),
    }
}
```

This locks us into these implementations. Testing requires:
- A running PostgreSQL
- Chromedp (which needs a browser)
- Environment variables to be set
- Possibly external services (email, webhook)

**Our way allows:**

```go
// In production
server := NewServer(
    postgres.NewStore(connStr),
    chromedp.NewConverter(),
)

// In unit tests
server := NewServer(
    &MockStore{},
    &MockConverter{},
)

// In integration tests
server := NewServer(
    memory.NewStore(),
    chromedp.NewConverter(),
)

// In local development
server := NewServer(
    sqlite.NewStore(":memory:"),
    mock.NewConverter(),
)
```

Same code. Different configurations. No recompilation needed if we use a config file.

## Pattern 3: The Repository Pattern

Our `DocumentStore` interface is a repository: a collection-like abstraction over persistence.

```go
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
}
```

**What this abstracts away:**

- Is data in memory, a database, or object storage?
- Are reads cached?
- Is there a write-ahead log?
- Is the system eventually consistent or strongly consistent?

The handler doesn't care. It just calls `store.GetDocumentJob()`.

### Implementation Changes: Transparent to Handlers

```go
// Today: In-memory
type MemoryStore struct {
    mu   sync.RWMutex
    jobs map[string]*Job
}

// Tomorrow: PostgreSQL
type PostgresStore struct {
    db *sql.DB
}

// Next year: DynamoDB
type DynamoStore struct {
    client *dynamodb.Client
}

// Handlers unchanged!
func (s *Server) HandleGetDocumentStatus(w http.ResponseWriter, r *http.Request) {
    job, err := s.store.GetDocumentJob(r.Context(), jobID)
    // Works the same with any implementation
}
```

This is the power of the repository pattern.

## Pattern 4: Constructor Injection

We don't use service locators or wire-up functions. We use explicit parameters:

```go
// ✓ GOOD: Constructor injection
func NewServer(store store.DocumentStore, converter converter.Converter) *Server {
    return &Server{
        store:     store,
        converter: converter,
    }
}

// ❌ BAD: Service locator
func NewServer() *Server {
    return &Server{
        store:     container.Get("DocumentStore"),
        converter: container.Get("Converter"),
    }
}

// ❌ BAD: Global variables
var globalStore store.DocumentStore
var globalConverter converter.Converter

func NewServer() *Server {
    return &Server{
        store:     globalStore,
        converter: globalConverter,
    }
}
```

**Why explicit parameters?**

1. **Visible dependencies**: You see what's needed just from the signature
2. **Type-safe**: Compiler catches wrong types
3. **Easy to test**: Just pass mocks directly
4. **Easy to parallelize tests**: No shared global state

## Pattern 5: Error Handling with Context

We pass `context.Context` to all storage operations:

```go
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    // Can check if context is cancelled
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    default:
    }
    // ... actual work
}
```

**Why context?**

1. **Cancellation**: If client disconnects, we can stop processing
2. **Timeouts**: Database query might timeout via context
3. **Tracing**: Context can carry distributed trace IDs
4. **Values**: Can pass request IDs, user info without method parameters

From the handler:

```go
func (s *Server) HandleCreateDocument(w http.ResponseWriter, r *http.Request) {
    // r.Context() is the request context
    // If client closes connection, ctx is cancelled
    jobID, err := s.store.CreateDocumentJob(r.Context(), file)
    if err != nil {
        // Handle error
    }
}
```

## Pattern 6: The Adapter Pattern (for Future Use)

When we implement the Converter, we'll likely use an external library. The adapter pattern isolates our code from library details:

```go
// ✓ GOOD: Our interface, external implementation
type chromedpConverter struct {
    browserCtx context.Context
}

func (c *chromedpConverter) MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error {
    // Adapt chromedp's API to our interface
    // Hide complexity here
}

// In main.go
converter := NewChromedpConverter(browserCtx)
server := NewServer(store, converter)  // Passes our interface
```

Benefits:
- If chromedp API changes, only this adapter needs updates
- If we replace chromedp with wkhtmltopdf, we implement a new adapter
- Handlers use our interface, untouched

## Pattern 7: The Factory Pattern (Deferred)

In `main.go`, we could create a factory function:

```go
func createStore(backend string) store.DocumentStore {
    switch backend {
    case "memory":
        return memory.NewStore()
    case "postgres":
        connStr := os.Getenv("DATABASE_URL")
        return postgres.NewStore(connStr)
    case "dynamodb":
        sess := session.Must(session.NewSession())
        return dynamodb.NewStore(sess)
    default:
        panic("unknown store backend: " + backend)
    }
}

func main() {
    backend := os.Getenv("STORE_BACKEND")  // From config/env
    store := createStore(backend)
    server := NewServer(store, converter)
}
```

**Why deferred?** We only build this complexity when we actually have multiple implementations. Avoid premature optimization.

## Thread Safety: Why RWMutex?

Our MemoryStore uses `sync.RWMutex`:

```go
type MemoryStore struct {
    mu   sync.RWMutex
    jobs map[string]*Job
}

func (ms *MemoryStore) GetDocumentJob(ctx context.Context, id string) (Job, error) {
    ms.mu.RLock()
    defer ms.mu.RUnlock()
    // ... multiple goroutines can be here simultaneously
}

func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    ms.mu.Lock()
    defer ms.mu.Unlock()
    // ... exclusive access, others must wait
}
```

**Performance implications:**

| Operation | Reader Lock Count | Writer Waits For | Writer Blocks |
|-----------|-------------------|------------------|---------------|
| Create Job | 1 (writing 1 job) | RLocks to finish | RLocks |
| Get Status | Many (checking) | Nothing | Locks |
| 100 GET, 1 POST | All 100 GETs proceed in parallel, POST waits for them | Perfect for read-heavy workloads |

This is why RWMutex is ideal for a store: far more reads than writes.

**In production** (with PostgreSQL), the database handles this.

## Error Handling: Fail Fast, Fail Loud

We handle errors at each layer:

```go
// Handler layer: HTTP concerns
func (s *Server) HandleCreateDocument(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        // Tell client what went wrong
    }
}

// Store layer: Business logic concerns
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    data, err := io.ReadAll(sourceFile)
    if err != nil {
        return "", fmt.Errorf("failed to read source file: %w", err)  // Wrap with context
    }
    
    if len(data) == 0 {
        return "", fmt.Errorf("source file is empty")  // Business rule
    }
}
```

**The `%w` verb** (Go 1.13+) is crucial:

```go
fmt.Errorf("failed to read source file: %w", err)  // ✓ Preserves error chain
fmt.Errorf("failed to read source file: %v", err)  // ✗ Loses error type
```

With `%w`, we can check the original error:

```go
err := store.CreateDocumentJob(ctx, file)
if errors.Is(err, io.EOF) {
    // Handle specific error
}

if errors.Is(err, io.ErrUnexpectedEOF) {
    // Handle another specific error
}
```

## Testing Strategy

Our design enables three levels of testing:

### Unit Tests: Interface + Mock

```go
func TestHandleCreateDocumentSuccess(t *testing.T) {
    mockStore := &MockStore{
        CreateFunc: func(ctx context.Context, sourceFile io.Reader) (string, error) {
            return "test-id", nil
        },
    }
    
    server := NewServer(mockStore, nil)
    // Test handler logic without real storage
}

func TestHandleCreateDocumentStorageError(t *testing.T) {
    mockStore := &MockStore{
        CreateFunc: func(ctx context.Context, sourceFile io.Reader) (string, error) {
            return "", fmt.Errorf("database unavailable")
        },
    }
    
    server := NewServer(mockStore, nil)
    // Test error handling
}
```

### Integration Tests: Real Store + Handler

```go
func TestCreateAndRetrieveJob(t *testing.T) {
    store := memory.NewStore()
    server := NewServer(store, nil)
    
    // Actually upload a file
    // Actually retrieve it
    // Verify end-to-end
}
```

### Acceptance Tests: Full HTTP Stack

```go
func TestDocumentUploadEndToEnd(t *testing.T) {
    store := memory.NewStore()
    server := NewServer(store, nil)
    
    router := chi.NewRouter()
    router.Post("/v1/documents", server.HandleCreateDocument)
    router.Get("/v1/documents/{id}", server.HandleGetDocumentStatus)
    
    // Make actual HTTP requests
    resp := makeRequest(router, "POST", "/v1/documents", ...)
    // Assert response
}
```

## Scalability Implications

Our architecture supports multiple deployment strategies:

### Single Instance (Today)

```
Client → HTTP API (single process)
         ↓
         MemoryStore
```

Simple, no network overhead, good for development.

### Distributed with Shared Storage (Tomorrow)

```
Client → Load Balancer → API Instance 1 ──┐
                         API Instance 2 ── PostgreSQL
                         API Instance 3 ──┘
```

All instances share PostgreSQL. MemoryStore replaced with PostgresStore. Code unchanged.

### Distributed with Event Queue (Later)

```
Client → Load Balancer → API Instance 1
                         API Instance 2  → RabbitMQ → Worker Queue → PDF Converter
                         API Instance 3

(API returns immediately, workers process asynchronously)
```

Job status changes flow through events. NewEventStore implements DocumentStore. Handlers unchanged.

## Performance Considerations

### Current (MemoryStore)

- **Time complexity**: O(1) for create and get (hash map lookup)
- **Space complexity**: O(n) where n = total jobs ever created
- **Concurrency**: RWMutex allows parallel reads
- **Bottleneck**: Single process, single machine

### Future (PostgreSQL)

- **Time complexity**: O(log n) with index on job ID
- **Space complexity**: Delegated to database
- **Concurrency**: Database handles it
- **Bottleneck**: Database connection pool

### With Caching (Redis)

- **Time complexity**: O(1) for cached reads
- **Overhead**: Cache invalidation logic
- **Benefit**: 100% faster status checks for hot jobs

## Backward Compatibility

Because we rely on interfaces, we can add features without breaking clients:

```go
// Today
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
}

// Tomorrow: Add new method
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
    DeleteDocumentJob(ctx context.Context, id string) error  // New!
}

// Existing implementations get compiler error
// They add the method (or return ErrNotSupported)
// Handlers can now optionally use it
```

This is interface evolution without breaking changes.

## Lessons from This Pattern

1. **Interfaces are about contracts, not implementation**: They describe behavior, not data structures.

2. **Dependency injection is about flexibility**: Pay a small verbosity cost now for large flexibility gains later.

3. **Separate concerns early**: When Store needs to become Store + Cache + Logger later, you can layer them.

4. **Make errors explicit**: Pass errors up with context, not error codes.

5. **Context is your friend**: Use it for cancellation, timeouts, and metadata.

6. **RWMutex is better than Mutex for read-heavy loads**: But only benchmark in your actual scenario.

## What's Next

In Part 3, we'll:

1. **Implement the Converter**: Use a real PDF library (or mock it)
2. **Add validation**: Check Markdown syntax
3. **Build comprehensive tests**: Unit + integration + acceptance
4. **Add logging**: Structured logging with context
5. **Deploy**: Docker container + local testing

---

*What design patterns have saved your projects? Share your experience in the comments.*
