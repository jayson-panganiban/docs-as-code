# Architecture Overview

## System Design

The docs-as-code API is built using clean architecture principles with clear separation of concerns. The system is designed to be:

- **Stateless**: No session state between requests
- **Scalable**: Horizontal scaling possible with persistent storage
- **Testable**: Dependency injection enables easy mocking
- **Maintainable**: Clear interfaces and responsibilities

## Component Diagram

```
┌─────────────────────────────────────────────────────┐
│         HTTP Request / Response                     │
└─────────────────────────┬───────────────────────────┘
                          │
                    ┌─────▼─────┐
                    │   Router   │ (chi)
                    └─────┬─────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
    ┌───▼──┐          ┌───▼──┐         ┌───▼──┐
    │POST  │          │GET   │    (Other routes)
    │/docs │          │/docs│
    └───┬──┘          └───┬──┘
        │                 │
    ┌───▼──────────────────▼───┐
    │     Server (Handler)     │ (internal/http)
    │  - HandleCreateDocument  │
    │  - HandleGetDocumentStatus│
    └───┬────────────┬──────────┘
        │            │
   ┌────▼─┐    ┌─────▼──────┐
   │Store │    │ Converter  │ (interfaces)
   └────┬─┘    └─────┬──────┘
        │            │
   ┌────▼──────────┐ │ ┌──────────────────┐
   │MemoryStore   │ └─┤PDFConverter      │
   │(development) │   │(gofpdf + goldmark)
   └──────────────┘   └──────────────────┘
```

## Layers

### 1. HTTP/Transport Layer (`internal/http`)

**Responsibility**: Handle HTTP protocol details

**Components**:
- `Server` struct: Holds dependencies and handler methods
- `HandleCreateDocument`: POST handler for document uploads
- `HandleGetDocumentStatus`: GET handler for job status
- `Response` struct: Standardized JSON response format

**Concerns**:
- Request parsing (multipart forms)
- Response serialization (JSON)
- HTTP status code selection
- Error handling and reporting

**Dependencies**: DocumentStore, Converter interfaces

### 2. Business Logic Layer

The business logic is split between two interfaces:

#### Document Store (`internal/store`)

**Responsibility**: Manage document job lifecycle and persistence

**Interface Methods**:
- `CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)`: Create a new job and return its ID
- `GetDocumentJob(ctx context.Context, id string) (Job, error)`: Retrieve job details by ID

**Current Implementation**: `MemoryStore`
- Stores jobs in a Go map
- Thread-safe with `sync.RWMutex`
- Generates UUIDs for job IDs
- Validates file content (rejects empty files)

**Future Implementations**:
- PostgreSQL backend
- MongoDB backend
- DynamoDB backend

#### Converter (`internal/converter`)

**Responsibility**: Transform Markdown to PDF format

**Interface Methods**:
- `MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error`: Read Markdown from input, write PDF to output

**Current Implementation**: `PDFConverter`
- Uses `goldmark` for Markdown parsing
- Uses `gofpdf` for PDF generation
- Renders HTML as PDF with basic formatting support
- Handles headings with font size variation

**Alternative Implementations**:
- chromedp-based converter (headless Chrome)
- wkhtmltopdf wrapper
- pandoc wrapper

### 3. Entry Point (`cmd/api`)

**Responsibility**: Application bootstrap and dependency wiring

**Concerns**:
- Parse configuration
- Initialize dependencies
- Set up routes
- Start HTTP server

**Current Flow**:
```
main()
  ├── Initialize DocumentStore (MemoryStore)
  ├── Initialize Converter (PDFConverter)
  ├── Create Server (dependency injection)
  ├── Set up chi Router
  ├── Register Routes
  └── Start HTTP Server
```

## Data Flow

### Document Creation Flow

```
Client Request
    │
    ▼
POST /v1/documents (multipart form)
    │
    ▼
Router (chi) routes to Server.HandleCreateDocument
    │
    ├─ Parse multipart form
    ├─ Extract "file" field
    ├─ Validate file exists
    │
    ▼
Server.HandleCreateDocument
    │
    ├─ Call DocumentStore.CreateDocumentJob(ctx, file)
    │   └─ Store.CreateDocumentJob:
    │       ├─ Read file content
    │       ├─ Validate (non-empty)
    │       ├─ Generate UUID
    │       ├─ Create Job struct
    │       ├─ Store in map (thread-safe)
    │       └─ Return job ID
    │
    ├─ Return 202 Accepted
    └─ Response body: {id, status="pending"}
```

### Status Check Flow

```
Client Request
    │
    ▼
GET /v1/documents/{id}
    │
    ▼
Router (chi) routes to Server.HandleGetDocumentStatus
    │
    ├─ Extract {id} path parameter
    ├─ Validate id not empty
    │
    ▼
Server.HandleGetDocumentStatus
    │
    ├─ Call DocumentStore.GetDocumentJob(ctx, id)
    │   └─ Store.GetDocumentJob:
    │       ├─ Lock RWMutex for reading
    │       ├─ Look up job in map
    │       ├─ Return job or error
    │       └─ Unlock
    │
    ├─ Return 200 OK
    └─ Response body: {id, status}
```

## Dependency Injection

Dependencies are injected at the HTTP server level:

```go
// In cmd/api/main.go
documentStore := store.NewMemoryStore()
pdfConverter := converter.NewPDFConverter()
server := httphandlers.NewServer(documentStore, pdfConverter)
```

Benefits:
1. **Testability**: Easy to pass mock implementations in tests
2. **Flexibility**: Can swap implementations at runtime
3. **Explicit Dependencies**: All required services are visible
4. **Loose Coupling**: Handlers don't know about concrete implementations

## Thread Safety

### MemoryStore Thread Safety

The `MemoryStore` uses `sync.RWMutex` to protect concurrent access:

```
CreateDocumentJob (write operation)
    └─ mu.Lock() → modify jobs map → mu.Unlock()

GetDocumentJob (read operation)
    └─ mu.RLock() → read from jobs map → mu.RUnlock()
```

This allows:
- Multiple concurrent reads
- Exclusive write access
- Safe concurrent access from HTTP handlers

### HTTP Handler Safety

Chi router creates a goroutine per request. Each handler:
1. Receives its own request/response writer
2. Calls thread-safe store methods
3. Has isolated request scope (no shared state)

## Error Handling Strategy

### Request-Level Errors

Handled in HTTP handlers with appropriate status codes:
- `400 Bad Request`: Malformed form, missing fields
- `404 Not Found`: Job doesn't exist
- `500 Internal Server Error`: Unexpected errors

### Store-Level Errors

Store methods return errors for:
- Empty file content
- Job not found
- Storage failures (future)

### Converter-Level Errors

Converter methods return errors for:
- Invalid Markdown (caught by goldmark parser)
- PDF generation failures (gofpdf rendering errors)
- File read failures
- Context cancellation

## Scalability Considerations

### Current Limitations (In-Memory Store)

- Data loss on restart
- Single instance only
- Memory usage grows with jobs
- No horizontal scaling

### Future Improvements

1. **Persistent Storage**: Replace MemoryStore with database backend
   - PostgreSQL for relational model
   - MongoDB for document model
   - DynamoDB for serverless scaling

2. **Async Processing**: Implement job queue
   - Redis Queue
   - RabbitMQ
   - Cloud Pub/Sub

3. **Caching**: Add Redis caching for frequently accessed jobs

4. **Load Balancing**: Multiple API instances behind load balancer

5. **Horizontal Scaling**: Stateless design enables scaling

## Testing Strategy

### Unit Tests

Test each component in isolation:
- Handler unit tests with mock store
- Store unit tests with in-memory implementation
- Converter unit tests with real implementation (see `pdf_converter_test.go`)

### Integration Tests

Test component interaction:
- Handler + Store integration
- Full request/response cycle

### Example Test Structure

```go
// handler_test.go
func TestHandleCreateDocument(t *testing.T) {
    mockStore := &MockStore{}
    server := NewServer(mockStore, nil)
    // ... test logic
}

// memory_store_test.go
func TestCreateDocumentJob(t *testing.T) {
    store := NewMemoryStore()
    jobID, err := store.CreateDocumentJob(ctx, reader)
    // ... assertions
}
```

## Security Considerations

### Current Implementation

- No authentication required
- No rate limiting
- No input validation beyond file type
- No CORS headers

### Future Enhancements

1. **Authentication**: API key or OAuth2
2. **Rate Limiting**: Per-IP or per-API-key limits
3. **Input Validation**: File type verification, size limits
4. **CORS**: Restrict cross-origin requests
5. **TLS/HTTPS**: Encrypt in-transit data
6. **Audit Logging**: Track all API calls

## Monitoring and Observability

### Current State

- Basic stdout logging in main.go

### Future Enhancements

1. **Structured Logging**: JSON logs with context
2. **Metrics**: Request counts, latencies, error rates
3. **Tracing**: Distributed tracing with context propagation
4. **Health Checks**: /health endpoint for monitoring
5. **Profiling**: pprof for performance analysis
