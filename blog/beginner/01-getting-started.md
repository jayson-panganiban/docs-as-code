# Building Your First Go Microservice: Part 1 - Getting Started

## Welcome!

You've decided to build a Go service. Congratulations! This series will guide you through building a real API, explaining every decision and pattern along the way.

By the end of this part, you'll understand:
- How to organize a Go project
- What an interface is and why you need one
- How to pass dependencies to your code
- How to write HTTP handlers that respond to requests

**No advanced Go knowledge required.** If you understand packages and functions, you're ready.

## What Are We Building?

We're building an API that:
1. Accepts files from users
2. Creates a "job" to convert them
3. Lets users check the status of their job

Think of it like a print queue: you submit a file, get a ticket number, and later ask "is my file done printing?"

### Real-World Uses

This pattern appears everywhere:
- Image upload services (resize and optimize)
- Video transcoding services
- Report generation services
- Email sending services
- Any task that takes time to complete

## Part 1: Creating the Project Structure

### What's a "Project Structure"?

When you write code, you organize it into folders and files. Good organization means:
- New developers understand how your code works
- You can find code easily
- Code doesn't get tangled together

Go has conventions (standard practices) for this. Following them makes your code professional and familiar to other Go developers.

### The Basic Structure

```
docs-as-code/
├── cmd/                          # Your applications
│   └── api/                      # The API application
│       └── main.go               # Starts the server
├── internal/                     # Private code (only for this project)
│   ├── http/                     # HTTP stuff (handlers, responses)
│   │   └── handlers.go           # Code that handles requests
│   ├── store/                    # Where we store jobs
│   │   └── store.go              # Job storage
│   └── converter/                # Converting files
│       ├── converter.go          # Conversion interface
│       ├── pdf_converter.go      # PDF converter implementation
│       └── pdf_converter_test.go # Converter tests
├── go.mod                        # What packages we depend on
├── go.sum                        # Lock file (trust me on this)
└── README.md                     # Documentation
```

### Why This Structure?

**cmd/** = Applications
- You might have multiple applications (`cmd/api`, `cmd/worker`, `cmd/cli`)
- Each has its own `main()` function
- Each is independently runnable

**internal/** = Your Secret Sauce
- Code here can't be imported by outsiders
- Forces good API boundaries
- Go will enforce this—try importing `internal/` and you'll get an error

**main.go** = The Starting Point
- Every Go program needs a `main()` function in a `main` package
- This is where your application starts running

Let's create this:

```bash
mkdir -p docs-as-code/cmd/api
mkdir -p docs-as-code/internal/http
mkdir -p docs-as-code/internal/store
mkdir -p docs-as-code/internal/converter
cd docs-as-code
```

## Part 2: Understanding Interfaces

### What's an Interface?

An interface is a **promise**. It says: "I will do these things."

Example from real life:
> A "Door" interface promises: you can open() and close()
> Any door (wooden, glass, electronic) keeps this promise

In Go:

```go
type Door interface {
    Open() error
    Close() error
}

type WoodenDoor struct {
    isOpen bool
}

func (d *WoodenDoor) Open() error {
    d.isOpen = true
    return nil
}

func (d *WoodenDoor) Close() error {
    d.isOpen = false
    return nil
}

// A GlassDoor also implements Door
type GlassDoor struct {
    isOpen bool
}

func (g *GlassDoor) Open() error {
    g.isOpen = true
    return nil
}

func (g *GlassDoor) Close() error {
    g.isOpen = false
    return nil
}

// This function works with ANY door!
func EnterRoom(door Door) error {
    return door.Open()
}
```

Notice: **I never told Go that WoodenDoor implements Door.** Go figures it out automatically. If WoodenDoor has `Open()` and `Close()` methods, it implements the Door interface.

### Why Interfaces Matter for Testing

Imagine your code says "I need a database." Without interfaces:

```go
// ❌ BAD: Hard-coded dependency
func CreateJob(db *PostgresDatabase) error {
    return db.Insert(...)
}

// Testing requires:
// - Installing PostgreSQL
// - Creating a test database
// - Managing cleanup
// Takes 30 seconds just to start a test!
```

With interfaces:

```go
// ✓ GOOD: Abstraction
type JobStore interface {
    SaveJob(job Job) error
}

func CreateJob(store JobStore) error {
    return store.SaveJob(job)
}

// Testing with a fake:
type FakeStore struct {
    savedJobs []Job
}

func (f *FakeStore) SaveJob(job Job) error {
    f.savedJobs = append(f.savedJobs, job)
    return nil
}

// Testing is instant, no setup needed!
func TestCreateJob(t *testing.T) {
    fake := &FakeStore{}
    err := CreateJob(fake)
    // ...
}
```

### Our Interfaces

For this project, we need two interfaces:

```go
// internal/store/store.go
// Promise: "I can save and retrieve jobs"
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
}

// internal/converter/converter.go
// Promise: "I can convert Markdown to PDF"
type Converter interface {
    MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}
```

**Don't worry about the specifics yet.** We're just defining what these things *promise* to do.

## Part 3: Understanding Dependency Injection

### What's Dependency Injection?

"Dependency Injection" sounds fancy, but it's simple:

> Instead of your code creating what it needs, someone gives it to you.

Example:

```go
// ❌ BAD: Creating dependencies inside
type ApiHandler struct {}

func (h *ApiHandler) HandleRequest() {
    db := CreateDatabase()              // Created inside
    converter := CreateConverter()      // Created inside
    // Use them...
}

// ✓ GOOD: Receiving dependencies
type ApiHandler struct {
    db        JobStore
    converter Converter
}

func (h *ApiHandler) HandleRequest() {
    // db and converter already exist
    // Just use them
}
```

### Why Inject Dependencies?

1. **Easy to test**: Pass fake implementations
2. **Easy to configure**: Use different implementations in different environments
3. **Explicit**: You see what the code needs by looking at the struct

### How to Inject

Use a constructor function (a function that creates your struct):

```go
type Server struct {
    store     JobStore
    converter Converter
}

// Constructor: Creates a Server with injected dependencies
func NewServer(store JobStore, converter Converter) *Server {
    return &Server{
        store:     store,
        converter: converter,
    }
}

// Usage:
func main() {
    store := CreateStore()              // Create in main()
    converter := CreateConverter()      // Create in main()
    
    // Inject them
    server := NewServer(store, converter)
    
    // Use server...
}
```

## Part 4: Writing Your First Handler

An HTTP handler is a function that responds to requests.

### The Simplest Handler

```go
package main

import (
    "fmt"
    "net/http"
)

// This is a handler: takes request, writes response
func HelloHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
}

func main() {
    // Register the handler
    http.HandleFunc("/", HelloHandler)
    
    // Start server on port 8080
    http.ListenAndServe(":8080", nil)
}
```

Let's break this down:

```go
func HelloHandler(w http.ResponseWriter, r *http.Request) {
    //            ↑                    ↑
    //         What we write to    The incoming request
    //         (the response)
}
```

`w` = ResponseWriter. It's a pipe to send data back to the client.
`r` = Request. It contains: URL, headers, body, method, etc.

### A More Realistic Handler

```go
func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    // Step 1: Get the file from the request
    file, err := r.FormFile("file")
    if err != nil {
        // Tell client: "I need a file"
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte("Please provide a file"))
        return
    }

    // Step 2: Use our injected store to create a job
    jobID, err := s.store.CreateJob(r.Context(), file)
    if err != nil {
        // Tell client: "Something went wrong on my end"
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte("Failed to create job"))
        return
    }

    // Step 3: Tell client: "Success!"
    w.WriteHeader(http.StatusAccepted)
    w.Write([]byte("Job created: " + jobID))
}
```

### Understanding HTTP Status Codes

Status codes tell the client "what happened":

| Code | Meaning | Example |
|------|---------|---------|
| 200 | OK, everything worked | Job status retrieved |
| 202 | Accepted, processing | Job created, will process |
| 400 | Bad Request, your fault | Missing required field |
| 404 | Not Found | Job ID doesn't exist |
| 500 | Server Error, my fault | Database crashed |

### The Handler Method Receiver

Notice the `(s *Server)` part?

```go
func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    // ↑ This is a METHOD, not a function
    // s is the receiver: the Server this method belongs to
    // Inside this method, s.store and s.converter are available
}
```

This means the handler can use anything on the Server struct:

```go
type Server struct {
    store     JobStore
    converter Converter
}

func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    // Can use s.store
    jobID, err := s.store.CreateJob(r.Context(), file)
    
    // Can use s.converter  
    err = s.converter.MarkdownToPDF(r.Context(), in, out)
}
```

## Part 5: Creating an In-Memory Store

A store is where you keep jobs. For now, we'll use a simple in-memory store (everything in RAM).

```go
// internal/store/memory_store.go

package store

import (
    "context"
    "io"
    "sync"
    
    "github.com/google/uuid"
)

type Job struct {
    ID     string  // Unique identifier
    Status string  // "pending", "done", etc.
}

type MemoryStore struct {
    mu   sync.RWMutex              // Lock for thread safety (explained later)
    jobs map[string]*Job           // All jobs, indexed by ID
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        jobs: make(map[string]*Job),  // Empty map
    }
}

// Implement the DocumentStore interface
func (ms *MemoryStore) CreateJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    // Step 1: Read the file
    data, err := io.ReadAll(sourceFile)
    if err != nil {
        return "", err  // Return error to caller
    }

    // Step 2: Validate it's not empty
    if len(data) == 0 {
        return "", fmt.Errorf("file is empty")
    }

    // Step 3: Create a unique ID
    jobID := uuid.New().String()

    // Step 4: Create the job
    job := &Job{
        ID:     jobID,
        Status: "pending",
    }

    // Step 5: Store it (with lock for safety)
    ms.mu.Lock()
    ms.jobs[jobID] = job
    ms.mu.Unlock()

    return jobID, nil
}

func (ms *MemoryStore) GetJob(ctx context.Context, id string) (Job, error) {
    // Read lock (multiple goroutines can read simultaneously)
    ms.mu.RLock()
    defer ms.mu.RUnlock()  // Always unlock, even if error

    job, found := ms.jobs[id]
    if !found {
        return Job{}, fmt.Errorf("job not found")
    }

    return *job, nil
}
```

### What Just Happened?

1. **NewMemoryStore()** = Constructor, creates empty store
2. **CreateJob()** = Implements our interface, creates jobs
3. **GetJob()** = Implements our interface, retrieves jobs
4. **Lock/Unlock** = Makes it safe for multiple requests to happen at once

Don't worry about locks yet. We'll explain in the next part.

## Part 6: Putting It Together in main.go

```go
// cmd/api/main.go

package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"

    "docs-as-code/internal/http"
    "docs-as-code/internal/store"
)

func main() {
    // Step 1: Create our store (where jobs live)
    jobStore := store.NewMemoryStore()

    // Step 2: Create our handler (the HTTP logic)
    handler := http.NewServer(jobStore, nil)
    // nil = no converter yet, we'll add later

    // Step 3: Create router (routes requests to handlers)
    router := chi.NewRouter()

    // Step 4: Register routes
    router.Post("/v1/documents", handler.HandleCreateJob)
    router.Get("/v1/documents/{id}", handler.HandleGetJobStatus)

    // Step 5: Start the server
    fmt.Println("Starting server on :8080")
    if err := http.ListenAndServe(":8080", router); err != nil {
        log.Fatal(err)
    }
}
```

### What's Happening Here?

**Step 1: Create the store**
```go
jobStore := store.NewMemoryStore()
```
This creates an empty job storage area in memory.

**Step 2: Create the handler**
```go
converter := converter.NewPDFConverter()  // Real converter now!
handler := http.NewServer(jobStore, converter)
```
This creates the thing that will handle requests. It gets:
- `jobStore`: where to save jobs
- `converter`: the PDF converter (now implemented with `goldmark` and `gofpdf`)

**Step 3: Create router**
```go
router := chi.NewRouter()
```
A router is like a receptionist: incoming requests come to the router, and it sends them to the right handler.

**Step 4: Register routes**
```go
router.Post("/v1/documents", handler.HandleCreateJob)
```
This says: "When someone POSTs to /v1/documents, call HandleCreateJob"

**Step 5: Start server**
```go
http.ListenAndServe(":8080", router)
```
This starts listening on port 8080 and passes requests to the router.

## Part 7: Testing What You Built

### Manually Testing

```bash
# Start the server
go run cmd/api/main.go

# In another terminal:

# Create a job
curl -X POST http://localhost:8080/v1/documents \
  -F "file=@README.md"

# You should get:
# {"id":"550e8400-e29b-41d4-a716-446655440000","status":"pending"}

# Check the status
curl http://localhost:8080/v1/documents/550e8400-e29b-41d4-a716-446655440000

# You should get:
# {"id":"550e8400-e29b-41d4-a716-446655440000","status":"pending"}
```

### Unit Testing

```go
// internal/http/handlers_test.go

package http

import (
    "strings"
    "testing"

    "docs-as-code/internal/store"
)

func TestCreateJob(t *testing.T) {
    // Create a real store
    s := store.NewMemoryStore()
    
    // Create a handler with that store
    handler := NewServer(s, nil)

    // TODO: Make a fake HTTP request
    // TODO: Call handler.HandleCreateJob()
    // TODO: Check the response
}
```

We're setting up the structure here. The actual test logic comes in Part 2.

## Design Decisions: Why We Did This

### Why Interfaces?

We defined `DocumentStore` as an interface so that:
- Later, we can swap `MemoryStore` for `PostgreSQLStore`
- Tests can use `FakeStore`
- Different parts of the code don't get tangled

### Why Dependency Injection?

We pass `store` to the handler so that:
- The handler doesn't know (or care) how storage works
- We can test with a fake store
- We can configure different stores in different environments

### Why This Project Structure?

Go has conventions:
- `cmd/` = applications
- `internal/` = private code
- Package names describe what they do (`http`, `store`, `converter`)

Following conventions means:
- Other Go developers understand your code immediately
- Tools work better (IDEs, go fmt, go vet)
- You avoid weird bugs from breaking conventions

### Why Memory Store First?

We start with MemoryStore because:
- It's simple (no database setup)
- It lets us test the whole API without external dependencies
- Later, we replace it with PostgreSQL/MongoDB/etc.

## Common Beginner Mistakes

### Mistake 1: Global State

```go
// ❌ DON'T DO THIS
var globalStore = store.NewMemoryStore()

func HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    globalStore.CreateJob(...)  // Hard to test!
}
```

**Why it's bad**: Tests interfere with each other. Hard to test with different stores.

**Do this instead**: Inject it.

### Mistake 2: Not Checking Errors

```go
// ❌ WRONG
jobID, _ := s.store.CreateJob(ctx, file)
w.Write([]byte("Job: " + jobID))

// ✓ RIGHT
jobID, err := s.store.CreateJob(ctx, file)
if err != nil {
    w.WriteHeader(http.StatusInternalServerError)
    w.Write([]byte("Error: " + err.Error()))
    return
}
w.Write([]byte("Job: " + jobID))
```

**Why it matters**: You need to know when things go wrong and tell the client.

### Mistake 3: Forgetting to Unlock Locks

```go
// ❌ WRONG
ms.mu.Lock()
// If you return here, lock is never released!
if job == nil {
    return nil, fmt.Errorf("not found")
}
// Lock held during expensive operation
doExpensiveThing()
ms.mu.Unlock()

// ✓ RIGHT
ms.mu.RLock()
defer ms.mu.RUnlock()  // Always unlocks, even on error
job, found := ms.jobs[id]
// Lock released immediately
// Safe to do expensive operations outside lock
```

## What You've Learned

✅ How to structure a Go project  
✅ What interfaces are and why they matter  
✅ What dependency injection is  
✅ How to write HTTP handlers  
✅ How to store data in memory  
✅ How to wire everything together  

## What's Next: Part 2

In Part 2, we'll:
- **Understand thread safety**: How to safely handle multiple requests
- **Write real tests**: Unit tests with mocks
- **Add JSON responses**: Making proper API responses
- **Handle errors better**: Giving clients helpful error messages

## Exercises

Try these to solidify your understanding:

1. **Add a new endpoint**: GET `/v1/documents` that lists all jobs
   - Hint: Create a new method on Server
   - Hint: Iterate over `s.store.jobs` (you'll need to expose it)

2. **Add job deletion**: DELETE `/v1/documents/{id}`
   - Add method to DocumentStore interface
   - Implement in MemoryStore
   - Add handler

3. **Add timestamps**: Store when jobs were created
   - Add `CreatedAt` field to Job struct
   - Set it in CreateJob
   - Return it in responses

## Resources

- [Go Official Tutorial](https://go.dev/doc/tutorial)
- [HTTP Package Docs](https://pkg.go.dev/net/http)
- [Interfaces](https://go.dev/tour/methods/9)
- [io.Reader/Writer](https://go.dev/blog/io-package)

---

**Ready to dive deeper? Move on to [Part 2: Threads, Tests & JSON](./02-threads-tests-json.md).**
