# Building a Docs-as-Code API: Part 1 - Initial Architecture & Scaffolding

## Introduction

In this series, we'll build a production-ready microservice from the ground up: a stateless HTTP API that accepts Markdown files, validates them, converts them to PDFs, and provides status tracking for conversion jobs. This is Part 1, where we establish the project structure, define core interfaces, and implement our first concrete store.

This journey documents real-world decisions in building scalable Go services: clean architecture, dependency injection, thread safety, and the path from prototype to production.

## The Challenge

We need to build an API that:

1. **Accepts file uploads**: Receive Markdown files via HTTP
2. **Processes asynchronously**: Create jobs and track their status
3. **Scales horizontally**: Stateless design, persistent storage
4. **Stays testable**: Swap implementations without code changes
5. **Remains simple**: Start small, grow as needed

The key insight: **avoid tying ourselves to implementation details too early**. Use interfaces to defer decisions about storage engines, PDF converters, and deployment strategies.

## Architecture: Interfaces First

Before writing a single line of code, we designed around interfaces:

```go
// internal/converter/converter.go
type Converter interface {
    MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}

// internal/store/store.go
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
}
```

**Why interfaces first?**

1. **Testing**: Mock these in tests without touching real storage
2. **Flexibility**: Swap PostgreSQL for MongoDB later without touching handlers
3. **Clarity**: The contract is explicit before implementation
4. **Teams**: Different developers can work on different implementations simultaneously

## Project Structure

We organized the code into clear layers:

```
cmd/api/
  └── main.go              # Entry point, dependency wiring

internal/
  ├── http/
  │   └── handlers.go      # HTTP request/response handling
  ├── store/
  │   ├── store.go         # Interface definitions
  │   └── memory_store.go  # Development implementation
  └── converter/
      ├── converter.go           # Interface definition
      ├── pdf_converter.go       # PDFConverter implementation
      └── pdf_converter_test.go  # Converter tests
```

**Why `internal/`?** Go's convention: code here can't be imported by external packages. Forces good API boundaries.

## The HTTP Handler: Dependency Injection in Action

Our `Server` struct holds all dependencies:

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

Notice: The handler doesn't create its dependencies. It receives them. This is **Constructor Injection**, the cleanest form of dependency injection in Go.

### Handling Document Creation

```go
func (s *Server) HandleCreateDocument(w http.ResponseWriter, r *http.Request) {
    // Parse multipart form
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "failed to parse form"})
        return
    }

    // Extract file
    file, _, err := r.FormFile("file")
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "file not provided"})
        return
    }
    defer file.Close()

    // Create job via store
    jobID, err := s.store.CreateDocumentJob(r.Context(), file)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(Response{Error: err.Error()})
        return
    }

    // Return 202 Accepted
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(Response{
        ID:     jobID,
        Status: "pending",
    })
}
```

**Key observations:**

1. **HTTP concerns** (parsing, status codes) live here
2. **Business logic** (job creation) delegates to store
3. **Clean error handling**: Each error level responds appropriately
4. **202 Accepted**: Async pattern—job created, processing not guaranteed yet

### Status Retrieval

```go
func (s *Server) HandleGetDocumentStatus(w http.ResponseWriter, r *http.Request) {
    jobID := chi.URLParam(r, "id")
    if jobID == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "missing job id"})
        return
    }

    job, err := s.store.GetDocumentJob(r.Context(), jobID)
    if err != nil {
        w.WriteHeader(http.StatusNotFound)
        json.NewEncoder(w).Encode(Response{Error: "job not found"})
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(Response{
        ID:     job.ID,
        Status: job.Status,
    })
}
```

Simple and focused: extract parameter, query store, return response.

## The MemoryStore: First Implementation

For development and testing, we built an in-memory store. It's not production-ready, but it's fully functional and teaches us about Go's concurrency primitives.

```go
type MemoryStore struct {
    mu   sync.RWMutex
    jobs map[string]*Job
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        jobs: make(map[string]*Job),
    }
}
```

### Creating a Job

```go
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    // Generate UUID
    jobID := uuid.New().String()

    // Read file
    data, err := io.ReadAll(sourceFile)
    if err != nil {
        return "", fmt.Errorf("failed to read source file: %w", err)
    }

    // Validate
    if len(data) == 0 {
        return "", fmt.Errorf("source file is empty")
    }

    // Store with write lock
    job := &Job{ID: jobID, Status: "pending"}
    ms.mu.Lock()
    ms.jobs[jobID] = job
    ms.mu.Unlock()

    return jobID, nil
}
```

**Why generate UUID?** To guarantee uniqueness across all instances and time. `github.com/google/uuid` handles this reliably.

### Retrieving a Job

```go
func (ms *MemoryStore) GetDocumentJob(ctx context.Context, id string) (Job, error) {
    ms.mu.RLock()
    defer ms.mu.RUnlock()

    job, exists := ms.jobs[id]
    if !exists {
        return Job{}, fmt.Errorf("job not found: %s", id)
    }

    return *job, nil
}
```

**The defer pattern**: Lock is guaranteed to release even if an error occurs. This is idiomatic Go.

## Thread Safety: RWMutex Explained

The `sync.RWMutex` is Go's readers-writer lock. It's crucial for concurrent safety:

```
Multiple goroutines reading simultaneously? ✓ RLock() allows this
One goroutine writing while others read? ✗ Lock() blocks all access
```

This is perfect for job stores: many status checks (reads), fewer job creations (writes).

```go
// Safe concurrent reads
go func() { store.GetDocumentJob(ctx, id1) }()
go func() { store.GetDocumentJob(ctx, id2) }()
go func() { store.GetDocumentJob(ctx, id3) }()

// All three read locks acquire simultaneously

// But this blocks until all readers finish:
go func() { store.CreateDocumentJob(ctx, file) }()
```

## Entry Point: Wiring It All Together

In `cmd/api/main.go`, we instantiate and wire everything:

```go
func main() {
    // Initialize dependencies
    documentStore := store.NewMemoryStore()
    
    // Create HTTP server
    server := httphandlers.NewServer(documentStore, nil)

    // Set up router
    router := chi.NewRouter()
    router.Post("/v1/documents", server.HandleCreateDocument)
    router.Get("/v1/documents/{id}", server.HandleGetDocumentStatus)

    // Start server
    fmt.Printf("Starting server on :8080\n")
    http.ListenAndServe(":8080", router)
}
```

**Update**: The converter is now fully implemented! We've added a `PDFConverter` that handles real Markdown-to-PDF conversion using `goldmark` and `gofpdf`. This shows how our interface-first design paid off: we could add the implementation without changing any of the handler or wiring code.

## Testing This Design

Here's how we'd test the handler without touching real storage:

```go
type MockStore struct {
    CreateFunc func(ctx context.Context, sourceFile io.Reader) (string, error)
    GetFunc    func(ctx context.Context, id string) (Job, error)
}

func (m *MockStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    return m.CreateFunc(ctx, sourceFile)
}

func (m *MockStore) GetDocumentJob(ctx context.Context, id string) (Job, error) {
    return m.GetFunc(ctx, id)
}

// In a test:
func TestCreateDocument(t *testing.T) {
    mockStore := &MockStore{
        CreateFunc: func(ctx context.Context, sourceFile io.Reader) (string, error) {
            return "test-id-123", nil
        },
    }
    
    server := NewServer(mockStore, nil)
    // Make request, assert response...
}
```

The handler doesn't know (or care) it's using a mock.

## Design Decisions: Why These Choices?

### 1. Interfaces Over Concrete Types

**Alternative**: Pass `*PostgresStore` directly to handlers.

**Why that's wrong**: 
- Forces PostgreSQL even in tests
- Can't use SQLite for local development
- Team member implementing Redis gets stuck

**Our choice**: Pass interface, swap implementations.

### 2. Constructor Injection

**Alternative**: Global variables or init() functions.

**Why that's wrong**:
- Global state is hard to test
- Order of initialization matters
- Can't run tests in parallel safely

**Our choice**: Explicit parameters = explicit dependencies.

### 3. 202 Accepted for Job Creation

**Alternative**: 201 Created with the job details.

**Why that's different**:
- 202 = "accepted for processing, might not be done yet"
- 201 = "created and complete"

Our async job model needs 202. If we later add queue-based processing, the response semantics remain correct.

### 4. UUID v4 for Job IDs

**Alternatives**: Sequential IDs, hash of content, timestamp + random.

**Why UUID v4**:
- Globally unique across all servers
- No coordination needed
- No leakage of sequential patterns
- Standard, well-understood format

## What We've Built

After this phase:

✓ **Project structure** follows Go conventions  
✓ **Interfaces** define contracts clearly  
✓ **HTTP handlers** manage protocol details  
✓ **In-memory store** lets us test the whole flow  
✓ **Thread-safe** operations with RWMutex  
✓ **Dependency injection** enables testing and flexibility  

## What's Next

In Part 2, we'll:

1. **Add tests**: Unit tests for handlers and store
2. **Implement PDF conversion**: The Converter interface
3. **Add validation**: Markdown syntax checking
4. **Implement middleware**: Logging, panic recovery
5. **Improve error handling**: Custom error types

## Lessons Learned

1. **Start with interfaces**: Defer implementation details. You don't need the perfect database on day one.

2. **Dependency injection is worth it**: The verbosity pays off in testability and flexibility.

3. **RWMutex is powerful**: For read-heavy workloads (like job status checks), it's more efficient than standard Mutex.

4. **HTTP semantics matter**: 202 Accepted tells clients "async, check back later." Use the right status codes.

5. **Error messages for debugging**: Include context in errors (`fmt.Errorf("job not found: %s", id)`) so operators know what happened.

## Code Review

Let's run the app:

```bash
$ go build -o bin/api cmd/api/main.go
$ ./bin/api
Starting server on :8080
```

Test it:

```bash
# Create a job
$ curl -X POST http://localhost:8080/v1/documents \
  -F "file=@README.md"

{
  "id":"550e8400-e29b-41d4-a716-446655440000",
  "status":"pending"
}

# Check status
$ curl http://localhost:8080/v1/documents/550e8400-e29b-41d4-a716-446655440000

{
  "id":"550e8400-e29b-41d4-a716-446655440000",
  "status":"pending"
}
```

It works. The foundation is solid.

## Conclusion

We've built a clean, testable, scalable foundation for a microservice. The interfaces give us flexibility. The dependency injection makes testing trivial. The in-memory store lets us validate the whole flow before committing to a database.

This is how you build production-ready Go services: start simple, make good architectural choices early, and keep the door open for complexity later.

**Next time**: We'll add real tests, implement PDF conversion, and make this service production-grade.

---

*Have questions about dependency injection, Go concurrency, or clean architecture? Leave a comment below.*
