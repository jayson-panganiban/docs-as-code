# Implementation Details

## File Structure

```
docs-as-code/
├── cmd/
│   └── api/
│       └── main.go                    # Application entry point
├── internal/
│   ├── converter/
│   │   ├── converter.go               # PDF converter interface
│   │   ├── pdf_converter.go           # PDFConverter implementation
│   │   └── pdf_converter_test.go      # Converter tests
│   ├── http/
│   │   └── handlers.go                # HTTP handlers and Server struct
│   └── store/
│       ├── store.go                   # Store interface and Job struct
│       └── memory_store.go            # In-memory implementation
├── docs/
│   ├── API.md                         # API reference
│   ├── ARCHITECTURE.md                # Architecture overview
│   └── IMPLEMENTATION.md              # This file
├── go.mod                             # Module definition
├── go.sum                             # Dependency lock file
├── README.md                          # Project overview
└── PROMPT.md                          # Original requirements
```

## Core Interfaces

### DocumentStore Interface

```go
type DocumentStore interface {
    CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJob(ctx context.Context, id string) (Job, error)
}
```

**Purpose**: Abstract document persistence

**Implementations**:
- `MemoryStore`: Current development implementation
- Future: PostgreSQL, MongoDB, DynamoDB

**Implementation Contract**:
- `CreateDocumentJob`: Read file, generate ID, store job, return ID
- `GetDocumentJob`: Look up job by ID, return Job struct or error
- Error for `GetDocumentJob` when job not found: `"job not found: {id}"`

### Converter Interface

```go
type Converter interface {
    MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}
```

**Purpose**: Abstract Markdown-to-PDF conversion

**Current Implementation**: `PDFConverter`
- Parses Markdown using `goldmark` library
- Renders to HTML intermediate format
- Generates PDF using `gofpdf` library
- Supports basic formatting (headers, lists, code blocks)

**Alternative Implementations**:
- chromedp (headless Chrome)
- wkhtmltopdf wrapper
- pandoc wrapper

**Implementation Contract**:
- Read Markdown from `in` reader
- Write PDF to `out` writer
- Return error if conversion fails
- Context can be used for cancellation

## Data Structures

### Job Struct

```go
type Job struct {
    ID     string  // Unique job identifier (UUID v4)
    Status string  // Job status: pending, processing, completed, failed
}
```

**Fields**:
- `ID`: Generated using `github.com/google/uuid`
- `Status`: String representing job state

**Status Values**:
- `pending`: Initial state, awaiting processing
- `processing`: Conversion in progress
- `completed`: Conversion succeeded
- `failed`: Conversion or validation failed

### Response Struct

```go
type Response struct {
    ID     string `json:"id,omitempty"`
    Status string `json:"status,omitempty"`
    Error  string `json:"error,omitempty"`
}
```

**JSON Serialization**:
- `omitempty` tags exclude empty fields from JSON output
- Success responses have `id` and `status`
- Error responses have `error` field

## MemoryStore Implementation

### Thread-Safe Storage

```go
type MemoryStore struct {
    mu   sync.RWMutex          // Protects concurrent access
    jobs map[string]*Job       // In-memory job storage
}
```

### Key Implementation Details

#### CreateDocumentJob

```go
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    // 1. Generate unique UUID
    jobID := uuid.New().String()
    
    // 2. Read entire file into memory
    data, err := io.ReadAll(sourceFile)
    if err != nil {
        return "", fmt.Errorf("failed to read source file: %w", err)
    }
    
    // 3. Validate file is not empty
    if len(data) == 0 {
        return "", fmt.Errorf("source file is empty")
    }
    
    // 4. Create Job struct
    job := &Job{
        ID:     jobID,
        Status: "pending",
    }
    
    // 5. Store with write lock
    ms.mu.Lock()
    ms.jobs[jobID] = job
    ms.mu.Unlock()
    
    return jobID, nil
}
```

**Key Points**:
- `uuid.New()` generates UUID v4 automatically
- `io.ReadAll()` reads entire file; consider streaming for large files
- Empty file validation prevents storage of invalid data
- Write lock protects map modification
- Returns `jobID` for client reference

#### GetDocumentJob

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

**Key Points**:
- Read lock allows concurrent reads
- `defer` ensures lock release even on error
- Returns copy of Job struct (value, not pointer)
- Error message format matches expected pattern

### Thread Safety Analysis

**Concurrent Read Safety**:
```
Goroutine A: GetDocumentJob() → RLock → Read map → RUnlock
Goroutine B: GetDocumentJob() → RLock → Read map → RUnlock
Result: Both proceed concurrently (safe)
```

**Read-Write Safety**:
```
Goroutine A: CreateDocumentJob() → Lock → Modify map → Unlock
Goroutine B: GetDocumentJob() → RLock → Read map → RUnlock
Result: Write blocks reads, reads block writes (safe)
```

## HTTP Handler Implementation

### Server Struct and Initialization

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

**Design Pattern**: Constructor injection ensures dependencies are provided

### HandleCreateDocument

**Request Processing**:
1. Parse multipart form (10 MB limit)
2. Extract "file" field
3. Close file handle (defer)
4. Call store.CreateDocumentJob()
5. Respond with job details

**Response Flow**:
- Success (202 Accepted): `{"id": "...", "status": "pending"}`
- Errors (400, 500): `{"error": "description"}`

**Error Handling Chain**:
```go
ParseMultipartForm() error
    ├─ 400: "failed to parse form: {error}"
    
FormFile("file") error
    ├─ 400: "file not provided or invalid"
    
store.CreateDocumentJob() error
    └─ 500: "failed to create job: {error}"
```

### HandleGetDocumentStatus

**Request Processing**:
1. Extract `id` from URL parameter (chi router)
2. Validate `id` is not empty
3. Call store.GetDocumentJob()
4. Respond with job status

**Response Flow**:
- Success (200 OK): `{"id": "...", "status": "pending"}`
- Not found (404): `{"error": "job not found"}`
- Errors (500): `{"error": "description"}`

## Router Configuration

### Chi Router Setup

```go
router := chi.NewRouter()

router.Post("/v1/documents", server.HandleCreateDocument)
router.Get("/v1/documents/{id}", server.HandleGetDocumentStatus)

http.ListenAndServe(":8080", router)
```

**Route Details**:
- POST handler: Creates new jobs
- GET handler: Retrieves job status
- Path parameter `{id}` extracted via `chi.URLParam(r, "id")`

## Dependency Management

### go.mod Structure

```
module docs-as-code
go 1.23

require (
    github.com/go-chi/chi/v5 v5.0.11
    github.com/google/uuid v1.5.0
)
```

### Dependency Usage

| Package | Purpose | Used In |
|---------|---------|---------|
| `chi/v5` | HTTP router | cmd/api/main.go |
| `google/uuid` | UUID generation | internal/store/memory_store.go |
| `goldmark` | Markdown parsing | internal/converter/pdf_converter.go |
| `gofpdf` | PDF generation | internal/converter/pdf_converter.go |

## Error Handling Pattern

### Error Types and Responses

```
Client Error (4xx)
├── 400 Bad Request
│   ├── Missing file field
│   ├── Malformed form data
│   ├── Missing path parameter
│
└── 404 Not Found
    └── Job not found by ID

Server Error (5xx)
└── 500 Internal Server Error
    ├── File read failure
    ├── Store operation failure
    └── Unexpected panic (needs middleware)
```

### Error Response Format

```go
Response{Error: "descriptive message"}
```

**Examples**:
- `"file not provided or invalid"`
- `"job not found"`
- `"failed to create job: source file is empty"`

## Testing Hooks

### Interface-Based Testing

MockStore example:
```go
type MockStore struct {
    CreateDocumentJobFunc func(ctx context.Context, sourceFile io.Reader) (string, error)
    GetDocumentJobFunc    func(ctx context.Context, id string) (Job, error)
}

func (m *MockStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
    return m.CreateDocumentJobFunc(ctx, sourceFile)
}
```

### Handler Testing Pattern

```go
func TestHandleCreateDocument(t *testing.T) {
    mockStore := &MockStore{
        CreateDocumentJobFunc: func(ctx context.Context, sourceFile io.Reader) (string, error) {
            return "test-id", nil
        },
    }
    
    mockConverter := &MockConverter{}  // Or use real converter
    server := NewServer(mockStore, mockConverter)
    // Create test request and response
    // Call server.HandleCreateDocument()
    // Assert response status and body
}
```

## PDF Converter Implementation Details

### PDFConverter Architecture

The `PDFConverter` implements a two-stage conversion pipeline:

1. **Markdown to HTML**:
   - Uses `goldmark` for RFC 7763 compliant Markdown parsing
   - Supports GitHub Flavored Markdown (GFM) via extension
   - Outputs HTML suitable for rendering

2. **HTML to PDF**:
   - Uses `gofpdf` for PDF generation
   - Strips HTML tags for display
   - Renders text with formatting:
     - Headings: Larger fonts (16pt H1, 14pt H2, 12pt H3)
     - Body: Standard 12pt Arial font
     - Lists: Inline bullet formatting
   - Handles page breaks automatically

### Testing

Comprehensive test coverage in `pdf_converter_test.go`:
- 5 functional test cases (simple markdown, formatting, lists, empty, code)
- HTML tag stripping tests
- PDF binary validation (checks for %PDF magic number)
- Benchmarking for performance analysis

## Future Implementation Considerations

### For Enhanced Converter Implementation

1. **Advanced Formatting**: Table rendering, embedded images
2. **Resource Limits**: Timeout context after N seconds
3. **Output Optimization**: Compression, font embedding
4. **Error Recovery**: Graceful fallback on parsing failures

### For Database Implementation

1. **Connection Pooling**: Manage DB connections
2. **Transactions**: Ensure job atomicity
3. **Indexing**: Speed up ID lookups
4. **Cleanup**: Implement job TTL/deletion

### For Middleware Addition

1. **Logging**: Log all requests with IDs
2. **Recovery**: Panic recovery to prevent crashes
3. **Validation**: Input type/size validation
4. **Metrics**: Request count/latency tracking
