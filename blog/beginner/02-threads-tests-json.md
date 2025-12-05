# Building Your First Go Microservice: Part 2 - Threads, Tests & JSON

## Introduction

In Part 1, you built a working API. But there are gaps:

1. What happens when two requests arrive at the same time?
2. How do you know your code actually works?
3. APIs should return JSON, not plain text

This part fills those gaps. By the end, you'll understand:
- Concurrency and why you need mutexes
- How to write tests
- How to return proper JSON responses
- How to handle errors properly

## Part 1: Understanding Goroutines & Concurrency

### The Problem: Multiple Requests at Once

When you start your server:

```go
http.ListenAndServe(":8080", router)
```

Go creates a new goroutine (lightweight thread) for **each request**. If 100 people request at the same time, you have 100 goroutines running simultaneously.

```
Request 1 ─┐
Request 2 ─┼─→ Memory Store  (But which one writes first?)
Request 3 ─┤
Request 4 ─┘
```

### The Data Race: A Real Problem

```go
type MemoryStore struct {
    jobs map[string]*Job   // Just a map
}

func (ms *MemoryStore) CreateJob(id string, job *Job) {
    ms.jobs[id] = job    // Goroutine A writes
}

func (ms *MemoryStore) GetJob(id string) *Job {
    return ms.jobs[id]   // Goroutine B reads
}
```

What if:
- Goroutine A is writing to the map
- Goroutine B reads from the map
- At the exact same moment?

Go panics. The map is corrupted.

```
Goroutine A: jobs[id] = job     ← Writing
Goroutine B: x = jobs[id]       ← Reading
               (CRASH!)
```

### The Solution: Mutex (Mutual Exclusion)

A mutex is a lock. Only one goroutine can hold it at a time.

```go
type MemoryStore struct {
    mu   sync.RWMutex            // Add a lock
    jobs map[string]*Job
}

func (ms *MemoryStore) CreateJob(id string, job *Job) {
    ms.mu.Lock()                 // Get exclusive lock
    ms.jobs[id] = job
    ms.mu.Unlock()               // Release lock
}

func (ms *MemoryStore) GetJob(id string) *Job {
    ms.mu.RLock()                // Get read-only lock
    defer ms.mu.RUnlock()        // Always release
    return ms.jobs[id]
}
```

Now:
```
Goroutine A: Lock()             ✓ Got it
Goroutine B: Lock() ⏳ Waiting...
Goroutine A: Write
Goroutine A: Unlock()           Release
Goroutine B: Lock() ✓ Got it now
Goroutine B: Read
Goroutine B: Unlock()
```

### RWMutex: Readers-Writer Lock

You might have noticed we used `RLock()` for reading:

```go
ms.mu.RLock()      // Read lock (shared)
ms.mu.Lock()       // Write lock (exclusive)
```

Here's why:

| Lock Type | Multiple Users? | Purpose |
|-----------|-----------------|---------|
| Lock() | ✗ One only | Exclusive access (writing) |
| RLock() | ✓ Many | Shared access (reading) |

If 100 requests are checking job status (reads), they can all do it simultaneously:

```
Goroutine A: RLock() ✓
Goroutine B: RLock() ✓ (can run same time!)
Goroutine C: RLock() ✓
...
Goroutine Z: CreateJob() Lock() ⏳ Waiting for all reads

Result: 26 status checks happen in parallel!
```

This is why RWMutex is better than regular Mutex for read-heavy operations.

### The defer Pattern: Safety Guarantee

```go
ms.mu.RLock()
defer ms.mu.RUnlock()   // Guaranteed to run, even on error

job, found := ms.jobs[id]
if !found {
    return nil, fmt.Errorf("not found")  // Lock still released!
}
return job, nil  // Lock still released!
```

`defer` runs when the function exits, no matter how. This prevents "lock leaks" (forgetting to unlock).

### Let's Trace a Real Scenario

```
Time  Goroutine A             Goroutine B             Jobs Map
────────────────────────────────────────────────────────────────
1     CreateJob("1", job1)
      mu.Lock()                                       [empty]

2                             GetJob("1")
                              mu.RLock() ⏳ WAITING...

3     jobs["1"] = job1                                jobs["1"]=job1

4     mu.Unlock()

5                             mu.RLock() ✓            
                              x = jobs["1"]          Read: job1

6                             mu.RUnlock()
```

Without the mutex, step 2 and 3 might happen simultaneously, corrupting the map.

## Part 2: Writing Tests

### Why Test?

Without tests, you have to:
1. Start the server manually
2. Use curl to test every scenario
3. Remember what you tested
4. Hope nothing broke when you changed code

Tests automate this. Change code, run tests instantly, sleep soundly.

### The Simplest Test

```go
// internal/store/memory_store_test.go

package store

import (
    "context"
    "strings"
    "testing"
)

func TestCreateJob(t *testing.T) {
    // Arrange: Set up
    store := NewMemoryStore()
    file := strings.NewReader("Hello, World!")

    // Act: Do the thing
    jobID, err := store.CreateJob(context.Background(), file)

    // Assert: Check the result
    if err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    if jobID == "" {
        t.Fatal("jobID is empty")
    }

    t.Logf("Created job with ID: %s", jobID)
}
```

Run it:

```bash
go test ./internal/store -v
```

### Understanding the Test Structure

```go
func TestCreateJob(t *testing.T) {
    //     ↑ Test functions start with "Test"
    //                    ↑ t is a testing helper
}
```

The `t` parameter lets you:
- `t.Fatalf()` = Stop test, print error (fail)
- `t.Errorf()` = Record error, keep going
- `t.Logf()` = Print debug info

### Test Multiple Scenarios

```go
func TestCreateJobEmpty(t *testing.T) {
    store := NewMemoryStore()
    
    // Empty file should fail
    file := strings.NewReader("")
    _, err := store.CreateJob(context.Background(), file)
    
    if err == nil {
        t.Fatal("Expected error for empty file, got nil")
    }
}

func TestGetJob(t *testing.T) {
    store := NewMemoryStore()
    
    // Create
    file := strings.NewReader("content")
    jobID, _ := store.CreateJob(context.Background(), file)
    
    // Retrieve
    job, err := store.GetJob(context.Background(), jobID)
    
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }
    
    if job.ID != jobID {
        t.Fatalf("Got wrong ID. Expected %s, got %s", jobID, job.ID)
    }
}

func TestGetJobNotFound(t *testing.T) {
    store := NewMemoryStore()
    
    // Try to get non-existent job
    _, err := store.GetJob(context.Background(), "nonexistent")
    
    if err == nil {
        t.Fatal("Expected error for nonexistent job, got nil")
    }
}
```

### Table-Driven Tests

When you have many similar test cases, use a table:

```go
func TestCreateJobScenarios(t *testing.T) {
    tests := []struct {
        name      string
        content   string
        wantError bool
    }{
        {"Empty file", "", true},
        {"Valid file", "Hello", false},
        {"Large file", strings.Repeat("x", 1000), false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := NewMemoryStore()
            file := strings.NewReader(tt.content)
            
            _, err := store.CreateJob(context.Background(), file)
            
            if (err != nil) != tt.wantError {
                t.Fatalf("Got error: %v, want error: %v", err != nil, tt.wantError)
            }
        })
    }
}
```

Run with: `go test -v ./internal/store`

### Testing Handlers (with Mocks)

The handler uses a store. We don't want to test the store again. We want a fake:

```go
// internal/http/handlers_test.go

package http

import (
    "context"
    "io"
    "testing"

    "docs-as-code/internal/store"
)

// FakeStore is a fake DocumentStore for testing
type FakeStore struct {
    createCalls int
    getCalls    int
}

func (f *FakeStore) CreateJob(ctx context.Context, file io.Reader) (string, error) {
    f.createCalls++
    return "fake-job-id", nil  // Always return success
}

func (f *FakeStore) GetJob(ctx context.Context, id string) (store.Job, error) {
    f.getCalls++
    return store.Job{ID: id, Status: "pending"}, nil
}

// Now test the handler
func TestHandleCreateJob(t *testing.T) {
    // Create a fake store
    fakeStore := &FakeStore{}
    
    // Create handler with fake store
    handler := NewServer(fakeStore, nil)
    
    // Create a fake HTTP request
    // (This is complex, we'll skip for now)
    
    // handler.HandleCreateJob(w, r)
    // Check the response
}
```

## Part 3: JSON Responses

### Why JSON?

Plain text is simple, but inflexible. APIs use JSON because:
- Structured (not just text)
- Standard format
- Easy to parse in any language

### Simple JSON Encoding

```go
import "encoding/json"

// Define what you're encoding
type Response struct {
    ID     string `json:"id"`
    Status string `json:"status"`
    Error  string `json:"error,omitempty"`  // omitempty = skip if empty
}

// Encode it
resp := Response{
    ID:     "job-123",
    Status: "pending",
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)

// Client receives:
// {"id":"job-123","status":"pending"}
```

The `json:"id"` part tells Go: "Call this field 'id' in JSON output."

### Error Responses

```go
func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    file, err := r.FormFile("file")
    if err != nil {
        // Return JSON error
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{
            Error: "file field required",
        })
        return
    }

    jobID, err := s.store.CreateJob(r.Context(), file)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(Response{
            Error: err.Error(),
        })
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)  // 202 for async
    json.NewEncoder(w).Encode(Response{
        ID:     jobID,
        Status: "pending",
    })
}
```

### Unmarshaling JSON (Parsing Input)

If your API receives JSON:

```go
type UploadRequest struct {
    Filename string `json:"filename"`
    Content  string `json:"content"`
}

var req UploadRequest
err := json.NewDecoder(r.Body).Decode(&req)
if err != nil {
    // JSON was invalid
}

// Use req.Filename and req.Content
```

## Part 4: Better Error Handling

### Custom Error Types

Instead of just returning `error`, create specific types:

```go
type JobNotFoundError struct {
    JobID string
}

func (e *JobNotFoundError) Error() string {
    return fmt.Sprintf("job not found: %s", e.JobID)
}

// Usage
func (ms *MemoryStore) GetJob(ctx context.Context, id string) (store.Job, error) {
    ms.mu.RLock()
    defer ms.mu.RUnlock()

    job, found := ms.jobs[id]
    if !found {
        return store.Job{}, &JobNotFoundError{JobID: id}
    }

    return *job, nil
}

// Checking for specific error type
err := store.GetJob(ctx, id)
var notFound *JobNotFoundError
if errors.As(err, &notFound) {
    w.WriteHeader(http.StatusNotFound)
} else {
    w.WriteHeader(http.StatusInternalServerError)
}
```

### Error Wrapping

When you encounter an error from another function, preserve it:

```go
// ❌ BAD: Lost information
data, err := io.ReadAll(file)
if err != nil {
    return "", fmt.Errorf("error")  // What error? Why?
}

// ✓ GOOD: Preserve original error
data, err := io.ReadAll(file)
if err != nil {
    return "", fmt.Errorf("failed to read file: %w", err)
    //                                          ↑
    //                              Preserves original
}
```

The `%w` verb keeps the original error. Callers can check it:

```go
err := store.CreateJob(ctx, file)
if errors.Is(err, io.EOF) {
    // Handle this specific error
}
```

## Part 5: Complete Handler Implementation

Now let's put it all together:

```go
// internal/http/handlers.go

package http

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"

    "docs-as-code/internal/store"
)

type Response struct {
    ID     string `json:"id,omitempty"`
    Status string `json:"status,omitempty"`
    Error  string `json:"error,omitempty"`
}

type Server struct {
    store store.DocumentStore
}

func NewServer(s store.DocumentStore) *Server {
    return &Server{store: s}
}

func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    // Always set content type
    w.Header().Set("Content-Type", "application/json")

    // Parse the uploaded file
    file, _, err := r.FormFile("file")
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{
            Error: "file field is required",
        })
        return
    }
    defer file.Close()

    // Create job
    jobID, err := s.store.CreateJob(r.Context(), file)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(Response{
            Error: err.Error(),
        })
        return
    }

    // Success
    w.WriteHeader(http.StatusAccepted)  // 202
    json.NewEncoder(w).Encode(Response{
        ID:     jobID,
        Status: "pending",
    })
}

func (s *Server) HandleGetJobStatus(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    jobID := chi.URLParam(r, "id")
    if jobID == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{
            Error: "job id is required",
        })
        return
    }

    job, err := s.store.GetJob(r.Context(), jobID)
    if err != nil {
        // Check if it's a not-found error
        var notFound *store.JobNotFoundError
        if errors.As(err, &notFound) {
            w.WriteHeader(http.StatusNotFound)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(Response{
            Error: err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(Response{
        ID:     job.ID,
        Status: job.Status,
    })
}
```

## Part 6: Thread Safety Checklist

Before you call your code "safe":

- [ ] All shared data has a mutex
- [ ] You lock before reading/modifying
- [ ] You defer unlock (always)
- [ ] You use RLock for reading
- [ ] You use Lock for writing
- [ ] You test with `-race` flag

```bash
go test -race ./...
```

If the race detector finds nothing, you're safe!

## Common Beginner Mistakes: Part 2

### Mistake 1: Forgetting defer

```go
// ❌ WRONG
ms.mu.Lock()
job, found := ms.jobs[id]
if !found {
    ms.mu.Unlock()  // Must remember!
    return nil, fmt.Errorf("not found")
}
// ... more code ...
ms.mu.Unlock()  // Repeated everywhere!

// ✓ RIGHT
ms.mu.Lock()
defer ms.mu.Unlock()  // Always runs
job, found := ms.jobs[id]
if !found {
    return nil, fmt.Errorf("not found")  // Unlock runs anyway
}
```

### Mistake 2: Testing Only Happy Path

```go
// ❌ INCOMPLETE: What if file is empty?
func TestCreateJob(t *testing.T) {
    store := NewMemoryStore()
    file := strings.NewReader("content")
    _, err := store.CreateJob(context.Background(), file)
    if err != nil {
        t.Fatal(err)
    }
    // That's it?!
}

// ✓ COMPLETE: Test many scenarios
func TestCreateJobEmpty(t *testing.T) { ... }
func TestCreateJobValid(t *testing.T) { ... }
func TestGetJobNotFound(t *testing.T) { ... }
// etc.
```

### Mistake 3: Wrong HTTP Status Code

```go
// ❌ WRONG: Server error for client mistake
if id == "" {
    w.WriteHeader(http.StatusInternalServerError)  // 500?
    return
}

// ✓ RIGHT: Client error for client mistake
if id == "" {
    w.WriteHeader(http.StatusBadRequest)  // 400
    return
}
```

### Mistake 4: Mutex in Wrong Scope

```go
// ❌ WRONG: Holds lock during I/O (slow!)
func (ms *MemoryStore) CreateJob(ctx context.Context, file io.Reader) (string, error) {
    ms.mu.Lock()
    data, err := io.ReadAll(file)  // Slow disk I/O while holding lock!
    job := &Job{...}
    ms.jobs[id] = job
    ms.mu.Unlock()
    return id, nil
}

// ✓ RIGHT: Lock only during critical section
func (ms *MemoryStore) CreateJob(ctx context.Context, file io.Reader) (string, error) {
    data, err := io.ReadAll(file)  // Fast: no lock held
    
    job := &Job{...}
    ms.mu.Lock()
    ms.jobs[id] = job  // Lock only here
    ms.mu.Unlock()
    return id, nil
}
```

## What You've Learned

✅ How goroutines and concurrency work  
✅ Why mutexes are necessary  
✅ How to write tests (unit and table-driven)  
✅ How to use mocks to test handlers  
✅ How to return JSON responses  
✅ How to handle errors properly  
✅ When to use RLock vs Lock  

## What's Next: Part 3

We'll implement:
- The actual Converter (Markdown to PDF)
- Logging (so you know what's happening)
- Graceful shutdown (clean server restart)
- Middleware (logging, metrics)
- Real database storage

## Exercises

1. **Add tests for all scenarios**:
   - Empty file
   - Valid file
   - Non-existent job
   - Concurrent creates

2. **Add logging**:
   - Log each request with the job ID
   - Log errors with context

3. **Add job listing endpoint**:
   - GET `/v1/documents`
   - Returns all jobs
   - Hint: Need to iterate ms.jobs safely

4. **Add status transitions**:
   - "pending" → "processing" → "completed"
   - Add UpdateJob method to store
   - Test the transitions

## Resources

- [sync Package Docs](https://pkg.go.dev/sync)
- [Testing in Go](https://golang.org/doc/effective_go#testing)
- [encoding/json](https://pkg.go.dev/encoding/json)
- [Error Handling](https://pkg.go.dev/errors)
- [Go Memory Model](https://golang.org/ref/mem)

---

**Ready for the real thing? Jump to [Part 3: PDF Conversion & Production Features](./03-pdf-conversion.md).**
