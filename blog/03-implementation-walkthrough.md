# Building a Docs-as-Code API: Part 3 - Implementation Walkthrough & Concurrency

## Overview

We've covered architecture and patterns. Now let's walk through the actual implementation details. How does the MemoryStore really work? What makes it thread-safe? How do we handle context cancellation? These are the questions that separate good Go developers from great ones.

## The MemoryStore: Under the Hood

### The Data Structure

```go
type MemoryStore struct {
    mu   sync.RWMutex          // Protects concurrent access
    jobs map[string]*Job       // In-memory job storage
}

type Job struct {
    ID     string              // UUID v4
    Status string              // State: pending, processing, completed, failed
}
```

Simple design: a mutex protecting a map. Let's see why this works.

### Creating a Job: Write Operation

```go
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    // Step 1: Generate UUID
    jobID := uuid.New().String()

    // Step 2: Read file into memory
    data, err := io.ReadAll(sourceFile)
    if err != nil {
        return "", fmt.Errorf("failed to read source file: %w", err)
    }

    // Step 3: Validate non-empty
    if len(data) == 0 {
        return "", fmt.Errorf("source file is empty")
    }

    // Step 4: Create job
    job := &Job{
        ID:     jobID,
        Status: "pending",
    }

    // Step 5: Store with exclusive lock
    ms.mu.Lock()
    ms.jobs[jobID] = job
    ms.mu.Unlock()

    return jobID, nil
}
```

**Timeline for concurrent CreateDocumentJob calls:**

```
Goroutine A (10:00:00.000)    Goroutine B (10:00:00.001)
───────────────────────────    ───────────────────────────
Generate UUID                  Generate UUID
Read file                       Read file
Validate                        Validate
Lock mu ✓ acquires              Lock mu ⏳ waits
Store in map                    Lock mu ✓ acquires
Unlock                          Store in map
Return jobID-A                  Unlock
                                Return jobID-B
```

**Why this order matters:**

We don't lock during file I/O (steps 1-4). This is crucial. If we did:

```go
// ❌ BAD: Lock during I/O
ms.mu.Lock()
data, err := io.ReadAll(sourceFile)  // Could take 5 seconds!
ms.mu.Unlock()
```

Other goroutines would block for 5 seconds waiting for the lock. Instead, we only lock during the actual critical section (map modification).

### Retrieving a Job: Read Operation

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

**Why read lock (RLock)?**

Multiple goroutines can hold RLock simultaneously:

```
Goroutine A: GetDocumentJob(id1) → RLock ✓ (acquires)
Goroutine B: GetDocumentJob(id2) → RLock ✓ (acquires)
Goroutine C: GetDocumentJob(id3) → RLock ✓ (acquires)

All three read concurrently!

Meanwhile:
Goroutine D: CreateDocumentJob() → Lock ⏳ (waits for A, B, C)
```

**The defer pattern:**

```go
ms.mu.RLock()
defer ms.mu.RUnlock()
```

This guarantees unlock even if:
- Early return happens
- Panic occurs
- An error is encountered

Go's defer is a safety net.

### The UUID Choice

```go
jobID := uuid.New().String()
```

This generates a UUID v4 (random). Example: `550e8400-e29b-41d4-a716-446655440000`

**Why UUID v4?**

| ID Type | Uniqueness | Scalability | Guess-ability | Sortability |
|---------|-----------|-------------|---------------|-------------|
| Sequential (1,2,3) | ✓ | ✗ (reveals count) | ✓ | ✓ |
| Timestamp-based | ✓ | ✓ | ✗ | ✓ |
| UUID v4 | ✓ | ✓ | ✗ | ✗ |
| Hash of content | ✗ (collisions) | ✓ | ✗ | ✗ |

We chose UUID v4 because:
1. **Distributed**: Works across all servers without coordination
2. **Unpredictable**: Can't guess next job ID (security)
3. **Standard**: Every language has UUID support
4. **Simple**: No database sequence management

Cost: Not sortable by creation time. If we need that later, we'd add `CreatedAt` field.

## HTTP Handler Implementation

### HandleCreateDocument: The Full Picture

```go
func (s *Server) HandleCreateDocument(w http.ResponseWriter, r *http.Request) {
    // Stage 1: Parse request
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "failed to parse form: " + err.Error()})
        return
    }

    // Stage 2: Extract file
    file, _, err := r.FormFile("file")
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "file not provided or invalid"})
        return
    }
    defer file.Close()

    // Stage 3: Create job
    jobID, err := s.store.CreateDocumentJob(r.Context(), file)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(Response{Error: "failed to create job: " + err.Error()})
        return
    }

    // Stage 4: Return success
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(Response{
        ID:     jobID,
        Status: "pending",
    })
}
```

Let's trace through a request:

#### Request arrives: Multipart form with file

```
POST /v1/documents HTTP/1.1
Content-Type: multipart/form-data; boundary=----Boundary

------Boundary
Content-Disposition: form-data; name="file"; filename="README.md"

# My Document
This is markdown...
------Boundary--
```

#### Stage 1: Parse Form

```go
if err := r.ParseMultipartForm(10 << 20); err != nil {
    // 10 << 20 = 10 * 2^20 = 10 MB
    // Files larger than 10 MB get rejected here
    // Prevents memory exhaustion from huge uploads
}
```

If parsing fails, we return 400 immediately.

#### Stage 2: Extract File

```go
file, _, err := r.FormFile("file")
// file is an io.Reader
// _ is FileHeader (name, size, etc.)
// err is non-nil if field doesn't exist
```

The file is now a reader we can pass to the store.

#### Stage 3: Create Job

```go
jobID, err := s.store.CreateDocumentJob(r.Context(), file)
```

Three things happen here:
1. Store reads the file
2. Store validates it
3. Store creates a job and stores it

If store returns error, it's probably data-related (empty file, I/O error), so 500 is appropriate.

#### Stage 4: Return Success

```go
w.WriteHeader(http.StatusAccepted)  // 202
json.NewEncoder(w).Encode(Response{
    ID:     jobID,
    Status: "pending",
})
```

202 Accepted tells the client: "I got your file, created a job, processing hasn't happened yet but will eventually."

**Why not 201 Created?**

201 implies the resource is ready to use. 202 is better for async workflows.

### HandleGetDocumentStatus: Minimal

```go
func (s *Server) HandleGetDocumentStatus(w http.ResponseWriter, r *http.Request) {
    jobID := chi.URLParam(r, "id")
    if jobID == "" {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "missing job id"})
        return
    }

    job, err := s.store.GetDocumentJob(r.Context(), jobID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusNotFound)
        json.NewEncoder(w).Encode(Response{Error: "job not found"})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(Response{
        ID:     job.ID,
        Status: job.Status,
    })
}
```

Simple flow: extract ID → query store → return result.

## Context: The Invisible Threading Through

Notice every function takes `context.Context`:

```go
CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
GetDocumentJob(ctx context.Context, id string) (Job, error)
HandleCreateDocument(w http.ResponseWriter, r *http.Request)
```

The handler's `r.Context()` flows through to store operations. Why?

### Scenario: Client Closes Connection

```
Client               Handler             Store
   │                  │                   │
   │ POST file        │                   │
   ├──────────────>   │                   │
   │                  │ CreateDocumentJob │
   │                  ├─────────────────> │
   X closes           │ (reads 10MB file) │
                      │                   │ (reading...)
```

Without context:
- Store might waste 30 seconds reading a file
- Handler returns to idle
- Client never sees the response

With context:

```go
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    // Periodically check: is context cancelled?
    data, err := io.ReadAll(sourceFile)
    
    // Can also wrap readers to respect context:
    select {
    case <-ctx.Done():
        return "", ctx.Err()  // Return immediately
    default:
        // proceed
    }
}
```

Chi router sets `r.Context()` to cancel when the client closes the connection. Our store can detect this and stop work.

### Scenario: Request Timeout

```go
// In middleware (future):
ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
defer cancel()

// Pass timeout context to store
job, err := s.store.GetDocumentJob(ctx, jobID)
// If 30 seconds elapse, ctx.Done() fires, operations cancel
```

Context makes timeouts automatic throughout the call stack.

## Error Handling: Semantic Errors

Our store returns errors with information:

```go
if len(data) == 0 {
    return "", fmt.Errorf("source file is empty")
}

if !exists {
    return Job{}, fmt.Errorf("job not found: %s", id)
}
```

The handler interprets these:

```go
err := s.store.GetDocumentJob(ctx, id)
if err != nil {
    // Is it "not found"?
    if strings.Contains(err.Error(), "not found") {
        w.WriteHeader(http.StatusNotFound)
    } else {
        w.WriteHeader(http.StatusInternalServerError)
    }
}
```

This works, but it's fragile. Better approach (used in production systems):

```go
type StorageError struct {
    Code    string  // "NOT_FOUND", "INVALID_INPUT", etc.
    Message string
    Err     error
}

func (e *StorageError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (ms *MemoryStore) GetDocumentJob(ctx context.Context, id string) (Job, error) {
    job, exists := ms.jobs[id]
    if !exists {
        return Job{}, &StorageError{
            Code:    "NOT_FOUND",
            Message: fmt.Sprintf("job not found: %s", id),
            Err:     errors.New("key missing from map"),
        }
    }
    return *job, nil
}

// Handler can check:
var storageErr *StorageError
if errors.As(err, &storageErr) {
    if storageErr.Code == "NOT_FOUND" {
        w.WriteHeader(http.StatusNotFound)
    }
}
```

This is more robust, but overkill for v1. Start simple, refactor when you need more.

## The Concurrency Test

How do we verify our RWMutex usage is correct? Stress testing:

```go
func TestConcurrentAccess(t *testing.T) {
    store := NewMemoryStore()
    done := make(chan bool, 100)

    // 50 goroutines creating jobs
    for i := 0; i < 50; i++ {
        go func() {
            _, err := store.CreateDocumentJob(
                context.Background(),
                strings.NewReader("test content"),
            )
            if err != nil {
                t.Errorf("Create failed: %v", err)
            }
            done <- true
        }()
    }

    // 50 goroutines reading jobs
    // (But we need to know job IDs... this is more complex)

    for i := 0; i < 100; i++ {
        <-done
    }
}
```

The real test is: run with `-race` flag:

```bash
go test -race ./internal/store
```

Go's race detector watches memory accesses. If one goroutine writes while another reads (without proper synchronization), it reports the race condition.

## File I/O Performance

```go
data, err := io.ReadAll(sourceFile)
```

This reads the entire file into memory. For our 10 MB limit, this is fine. But what about edge cases?

### Sparse file (mostly zeros)

Go's `io.ReadAll()` still allocates the full size. For a 10 MB sparse file:

```
User: $ ls -lh large_file.md
-rw-r--r--  1 user  staff  10M Jan  1 00:00 large_file.md

Server: ReadAll() allocates 10 MB in memory
```

In production with PostgreSQL and streaming uploads, we'd use:

```go
func (ps *PostgresStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    jobID := uuid.New().String()
    
    // Stream to database, don't buffer entirely
    _, err := io.CopyN(db.GetWriter(jobID), sourceFile, 10<<20)
    if err != nil {
        return "", fmt.Errorf("file too large or read failed: %w", err)
    }
    
    return jobID, nil
}
```

But for in-memory, we have to buffer.

## The Full Request Lifecycle

Let's trace a complete request:

### Request 1: Upload file

```
Client                        Handler                Store
  │                            │                      │
  │ POST /v1/documents         │                      │
  │ (multipart with README.md) │                      │
  │ ─────────────────────────> │                      │
  │                            │                      │
  │                            ParseMultipartForm()   │
  │                            ✓ Form parsed          │
  │                            │                      │
  │                            FormFile("file")       │
  │                            ✓ File reader ready    │
  │                            │                      │
  │                            CreateDocumentJob()    │
  │                            │                      │
  │                            ├─ Read file into mem  │
  │                            ├─ Validate (non-empty)│
  │                            │                      │
  │                            │ Lock mu              │
  │                            │ Store job            │
  │                            │ Unlock mu            │
  │                            │                      │
  │                            return "job-123"       │
  │                            │                      │
  │ 202 Accepted               │                      │
  │ {"id":"job-123",           │                      │
  │  "status":"pending"}       │                      │
  │ <───────────────────────── │                      │
  │                            │                      │
```

### Request 2: Check status

```
Client                        Handler                Store
  │                            │                      │
  │ GET /v1/documents/job-123  │                      │
  │ ──────────────────────────>│                      │
  │                            │                      │
  │                            URLParam(r, "id")      │
  │                            ✓ id = "job-123"       │
  │                            │                      │
  │                            GetDocumentJob()       │
  │                            │                      │
  │                            │ RLock mu (shared)   │
  │                            │ Look up "job-123"    │
  │                            │ RUnlock mu           │
  │                            │                      │
  │                            return Job{...}        │
  │                            │                      │
  │ 200 OK                     │                      │
  │ {"id":"job-123",           │                      │
  │  "status":"pending"}       │                      │
  │ <───────────────────────── │                      │
  │                            │                      │
```

## Lessons from Implementation

1. **Lock granularity matters**: Lock only during critical sections, not during I/O
2. **RWMutex is powerful**: For read-heavy stores, it's a game-changer
3. **Context is essential**: Always pass context through your call stack
4. **Error messages matter**: Include context so operators can debug
5. **Defer is your friend**: Guarantees cleanup even during errors
6. **Test with -race**: Concurrency bugs are subtle; let the race detector find them

## What's Next: Part 4

We'll tackle:
1. **Testing strategies**: Unit tests with mocks, integration tests, concurrent tests
2. **The Converter interface**: Implementing markdown-to-PDF
3. **Adding middleware**: Logging and error recovery
4. **Deployment**: Docker, graceful shutdown, health checks

---

*What concurrency patterns have you found invaluable in Go? Let me know in the comments.*
