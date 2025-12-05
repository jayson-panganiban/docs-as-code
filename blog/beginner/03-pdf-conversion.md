# Building Your First Go Microservice: Part 3 - PDF Conversion & Production Features

## Introduction

So far, you have:
- ✓ A working API that accepts files
- ✓ Job storage and retrieval
- ✓ Thread-safe operations
- ✓ Tests and proper error handling

But your API doesn't actually *convert* anything. And it'll crash if someone sends bad requests. This part fixes both issues.

By the end, you'll understand:
- How to integrate external libraries
- How to wrap them with your interfaces
- How to add logging
- How to handle graceful shutdown
- How to make production-ready code

## Part 1: Implementing the Converter

### The Converter Interface (Recap)

Remember, we defined:

```go
type Converter interface {
    MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}
```

Now let's implement it.

### Option 1: Simple Implementation (Mock)

For testing, a mock converter is perfect:

```go
// internal/converter/mock_converter.go

package converter

import (
    "context"
    "io"
)

type MockConverter struct {
    // Track how many times it was called
    CallCount int
}

func (m *MockConverter) MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error {
    m.CallCount++
    
    // Just write fake PDF bytes
    _, err := io.WriteString(out, "%PDF-1.4\n[fake pdf content]\n")
    return err
}
```

Usage in code:

```go
func main() {
    store := store.NewMemoryStore()
    converter := &converter.MockConverter{}  // Fake!
    server := http.NewServer(store, converter)
    // ...
}
```

### Option 2: Real Implementation (Placeholder)

For now, a simple implementation:

```go
// internal/converter/simple_converter.go

package converter

import (
    "context"
    "fmt"
    "io"
)

type SimpleConverter struct {
    // You could add configuration here
}

func NewSimpleConverter() *SimpleConverter {
    return &SimpleConverter{}
}

func (s *SimpleConverter) MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error {
    // Read markdown
    data, err := io.ReadAll(in)
    if err != nil {
        return fmt.Errorf("failed to read markdown: %w", err)
    }

    // For now, just write fake PDF
    // In production, you'd use a real library here
    output := fmt.Sprintf("PDF conversion of: %s\n", string(data))
    
    _, err = io.WriteString(out, output)
    return err
}
```

### Option 3: Real PDF Library (Advanced)

Here's how you'd use a real library. This uses `github.com/go-echarts/go-echarts` as an example (you'd choose an actual PDF library):

```go
// internal/converter/pdf_converter.go

package converter

import (
    "context"
    "io"
    
    // Hypothetical library
    // In reality, you might use: chromedp, pdfcpu, or others
)

type PDFConverter struct {
    // Configuration for the converter
    basePath string
}

func NewPDFConverter(basePath string) *PDFConverter {
    return &PDFConverter{basePath: basePath}
}

func (p *PDFConverter) MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error {
    // Step 1: Read markdown
    data, err := io.ReadAll(in)
    if err != nil {
        return fmt.Errorf("failed to read markdown: %w", err)
    }

    // Step 2: Validate it's actually markdown
    // (Check for markdown indicators, etc.)

    // Step 3: Convert using external library
    // pdf := convertMarkdownToPDF(string(data))

    // Step 4: Write to output
    // _, err := out.Write(pdf)
    // return err

    // Placeholder for now
    return fmt.Errorf("PDF converter not yet implemented")
}
```

## Part 2: Adding Logging

### Why Logging?

Without logging, you have no idea what's happening:
- Did the request arrive?
- Why did it fail?
- How long did it take?

Logs tell the story.

### Simple Logging

The standard library has `log`:

```go
import "log"

log.Println("Server started")
log.Printf("Created job: %s\n", jobID)
log.Printf("ERROR: Failed to convert: %v\n", err)
```

But for production, use structured logging (logs as JSON):

```go
import "log/slog"

slog.Info("server started", "port", 8080)
slog.Error("conversion failed", "jobID", jobID, "error", err)
```

### Logging in Handlers

```go
func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    slog.Info("POST /v1/documents", "method", r.Method, "path", r.URL.Path)

    file, _, err := r.FormFile("file")
    if err != nil {
        slog.Warn("no file provided", "error", err)
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(Response{Error: "file required"})
        return
    }
    defer file.Close()

    jobID, err := s.store.CreateJob(r.Context(), file)
    if err != nil {
        slog.Error("failed to create job", "error", err)
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(Response{Error: err.Error()})
        return
    }

    slog.Info("job created", "jobID", jobID)
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(Response{ID: jobID, Status: "pending"})
}
```

### Logging Middleware

A middleware runs before your handler. Perfect for logging:

```go
// internal/http/middleware.go

package http

import (
    "log/slog"
    "net/http"
    "time"
)

// LoggingMiddleware logs each request
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Call the next handler
        next.ServeHTTP(w, r)

        // Log the request
        duration := time.Since(start)
        slog.Info("request", 
            "method", r.Method,
            "path", r.URL.Path,
            "duration", duration.String(),
        )
    })
}

// Usage in main.go:
router := chi.NewRouter()
router.Use(LoggingMiddleware)  // Apply to all routes
```

## Part 3: Graceful Shutdown

### The Problem

Your server is running. You deploy a new version. You kill the server.

But what if a request is being processed? It crashes mid-operation.

### The Solution: Graceful Shutdown

Tell the server "stop accepting new requests, but finish existing ones":

```go
// cmd/api/main.go

package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"

    httphandlers "docs-as-code/internal/http"
    "docs-as-code/internal/store"
)

func main() {
    // Create server
    store := store.NewMemoryStore()
    handler := httphandlers.NewServer(store, nil)

    router := chi.NewRouter()
    router.Post("/v1/documents", handler.HandleCreateJob)
    router.Get("/v1/documents/{id}", handler.HandleGetJobStatus)

    // Start in a goroutine
    server := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    go func() {
        slog.Info("Starting server on :8080")
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server error", "error", err)
        }
    }()

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    <-sigChan  // Block until signal received
    slog.Info("Shutdown signal received")

    // Graceful shutdown: finish existing requests, timeout after 10 seconds
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        slog.Error("shutdown error", "error", err)
    }

    slog.Info("Server shutdown complete")
}
```

What happens:

1. Server is running
2. You press Ctrl+C
3. `signal.Notify` receives it
4. `server.Shutdown()` is called
5. Server stops accepting new connections
6. Existing requests finish (up to 10 seconds)
7. Server exits cleanly

### Testing Graceful Shutdown

```bash
# Start server
go run cmd/api/main.go &
SERVER_PID=$!

# Make a request that takes time
curl -X POST http://localhost:8080/v1/documents &

# Give it a second
sleep 1

# Kill the server
kill $SERVER_PID

# Server should wait for the request to finish
# Then exit cleanly
```

## Part 4: Environment Configuration

### The Problem

Your code has hardcoded values:

```go
http.ListenAndServe(":8080", router)  // Always port 8080
```

But maybe:
- Development needs :8080
- Testing needs :8081
- Production needs :5000

### The Solution: Environment Variables

```go
// cmd/api/main.go

package main

import (
    "os"
)

func main() {
    // Get port from environment, default to 8080
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    // Get log level
    logLevel := os.Getenv("LOG_LEVEL")
    if logLevel == "" {
        logLevel = "info"
    }

    // ... setup logging with logLevel ...

    addr := ":" + port
    server := &http.Server{
        Addr:    addr,
        Handler: router,
    }

    slog.Info("Starting server", "addr", addr, "logLevel", logLevel)
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        slog.Error("server error", "error", err)
    }
}
```

Usage:

```bash
# Default port
go run cmd/api/main.go

# Custom port
PORT=5000 go run cmd/api/main.go

# Docker
docker run -e PORT=5000 -e LOG_LEVEL=debug myapp
```

## Part 5: Health Checks

### Why Health Checks?

Load balancers need to know: "Is this server healthy?"

Without health checks, they might route requests to a dead server.

### Simple Health Check

```go
// internal/http/handlers.go

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
    })
}

// main.go
router.Get("/health", handler.HandleHealth)
```

Usage:

```bash
curl http://localhost:8080/health
# {"status":"healthy"}
```

### Advanced Health Check

Check if dependencies are healthy:

```go
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    // Check store
    _, err := s.store.GetJob(r.Context(), "test-nonexistent")
    storeHealthy := err != nil  // Expected error = store is working

    // Check converter
    converterHealthy := s.converter != nil  // Converter exists

    if storeHealthy && converterHealthy {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "healthy",
            "store":  "ok",
            "converter": "ok",
        })
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "unhealthy",
            "store":  storeHealthy,
            "converter": converterHealthy,
        })
    }
}
```

## Part 6: Better Error Types

Instead of returning strings, return structured errors:

```go
// internal/store/errors.go

package store

type JobNotFoundError struct {
    JobID string
}

func (e *JobNotFoundError) Error() string {
    return "job not found: " + e.JobID
}

type InvalidJobError struct {
    Reason string
}

func (e *InvalidJobError) Error() string {
    return "invalid job: " + e.Reason
}

// Usage
func (ms *MemoryStore) GetJob(ctx context.Context, id string) (Job, error) {
    ms.mu.RLock()
    defer ms.mu.RUnlock()

    job, found := ms.jobs[id]
    if !found {
        return Job{}, &JobNotFoundError{JobID: id}
    }

    return *job, nil
}

// Handler can check error type
import "errors"

err := s.store.GetJob(ctx, id)
var notFound *JobNotFoundError
if errors.As(err, &notFound) {
    w.WriteHeader(http.StatusNotFound)
} else {
    w.WriteHeader(http.StatusInternalServerError)
}
```

## Part 7: Integration Testing

Unit tests test single functions. Integration tests test the whole flow:

```go
// internal/http/handlers_integration_test.go

package http

import (
    "bytes"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/go-chi/chi/v5"

    "docs-as-code/internal/store"
)

func TestCreateAndRetrieveJob(t *testing.T) {
    // Create real store
    store := store.NewMemoryStore()
    
    // Create handler
    handler := NewServer(store, nil)
    
    // Create router
    router := chi.NewRouter()
    router.Post("/v1/documents", handler.HandleCreateJob)
    router.Get("/v1/documents/{id}", handler.HandleGetJobStatus)

    // Create a fake HTTP request
    body := strings.NewReader("test file content")
    req := httptest.NewRequest("POST", "/v1/documents", body)
    req.Header.Set("Content-Type", "multipart/form-data")
    
    // Add file to request (simplified)
    // In real code, use mime/multipart package

    // Execute request
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    // Check response
    if w.Code != http.StatusAccepted {
        t.Fatalf("Expected 202, got %d", w.Code)
    }

    // Parse response
    var resp Response
    json.NewDecoder(w.Body).Decode(&resp)

    // Retrieve the job
    req2 := httptest.NewRequest("GET", "/v1/documents/"+resp.ID, nil)
    w2 := httptest.NewRecorder()
    router.ServeHTTP(w2, req2)

    if w2.Code != http.StatusOK {
        t.Fatalf("Expected 200, got %d", w2.Code)
    }

    var resp2 Response
    json.NewDecoder(w2.Body).Decode(&resp2)

    if resp2.ID != resp.ID {
        t.Fatalf("Expected ID %s, got %s", resp.ID, resp2.ID)
    }
}
```

## Part 8: Complete main.go

Here's a production-ready main.go:

```go
// cmd/api/main.go

package main

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"

    httphandlers "docs-as-code/internal/http"
    "docs-as-code/internal/converter"
    "docs-as-code/internal/store"
)

func main() {
    // Configure logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    // Get configuration from environment
    port := getEnv("PORT", "8080")
    logLevel := getEnv("LOG_LEVEL", "info")
    
    slog.Info("Starting server",
        "port", port,
        "logLevel", logLevel,
    )

    // Create dependencies
    documentStore := store.NewMemoryStore()
    pdfConverter := converter.NewSimpleConverter()

    // Create handler
    server := httphandlers.NewServer(documentStore, pdfConverter)

    // Set up router
    router := chi.NewRouter()
    router.Use(httphandlers.LoggingMiddleware)

    router.Get("/health", server.HandleHealth)
    router.Post("/v1/documents", server.HandleCreateJob)
    router.Get("/v1/documents/{id}", server.HandleGetJobStatus)

    // Create HTTP server
    httpServer := &http.Server{
        Addr:    ":" + port,
        Handler: router,
    }

    // Start in goroutine
    go func() {
        slog.Info("HTTP server listening", "addr", httpServer.Addr)
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server error", "error", err)
            os.Exit(1)
        }
    }()

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    <-sigChan
    slog.Info("shutdown signal received")

    // Graceful shutdown (10 second timeout)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := httpServer.Shutdown(ctx); err != nil {
        slog.Error("shutdown failed", "error", err)
        os.Exit(1)
    }

    slog.Info("server shutdown complete")
}

func getEnv(key, defaultValue string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultValue
}
```

## Production Checklist

Before shipping to production:

- [ ] Logging is in place (structured logs)
- [ ] Graceful shutdown works
- [ ] Health checks pass
- [ ] Environment variables configured
- [ ] Error handling is comprehensive
- [ ] Tests pass (unit + integration)
- [ ] Race conditions eliminated (`go test -race`)
- [ ] Performance tested (load test)
- [ ] Dependencies pinned (go.sum)
- [ ] README updated
- [ ] API documented

## Common Production Issues

### Issue 1: File Descriptors Leak

```go
// ❌ WRONG: File never closed
file, _ := r.FormFile("file")
s.store.CreateJob(ctx, file)
// file is left open!

// ✓ RIGHT
file, _ := r.FormFile("file")
defer file.Close()
s.store.CreateJob(ctx, file)
```

### Issue 2: No Request Timeout

```go
// ❌ WRONG: Request could take forever
resp, err := http.Post(url, "application/json", body)

// ✓ RIGHT: 30 second timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

client := &http.Client{}
req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
resp, err := client.Do(req)
```

### Issue 3: No Connection Pooling

Database connections are expensive. Reuse them:

```go
// Create once in main()
db := sql.Open("postgres", connStr)
defer db.Close()

// Reuse for all requests
```

## What You've Learned

✅ How to implement interfaces  
✅ How to add logging  
✅ How to implement graceful shutdown  
✅ How to use environment variables  
✅ How to add health checks  
✅ How to structure error types  
✅ How to write integration tests  
✅ Production best practices  

## What's Next: Part 4 (Next Steps)

When you're ready:
- **Database**: Switch from MemoryStore to PostgreSQL
- **Docker**: Containerize your app
- **Kubernetes**: Deploy to production
- **Monitoring**: Add Prometheus metrics
- **Caching**: Add Redis for performance
- **Real PDF**: Implement actual Markdown to PDF conversion

## Exercises

1. **Add PDF generation**:
   - Use a library to actually convert Markdown
   - Test it with real files

2. **Add request validation**:
   - Check file is actually Markdown
   - Limit file size

3. **Add metrics**:
   - Count total jobs created
   - Track conversion duration
   - Log to Prometheus format

4. **Add database storage**:
   - Implement PostgreSQLStore
   - Use same DocumentStore interface
   - Jobs persist across restarts

5. **Containerize**:
   - Create Dockerfile
   - Build and run with Docker
   - Test with docker-compose

## Resources

- [log/slog Package](https://pkg.go.dev/log/slog)
- [Graceful Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)
- [Integration Testing](https://golang.org/doc/effective_go#testing)
- [envconfig Library](https://github.com/kelseyhightower/envconfig)
- [Chi Router](https://github.com/go-chi/chi)

---

**You've built a production-grade API! Consider deploying it and monitoring it in the real world.**

*Share your project in the comments. What would you build next?*
