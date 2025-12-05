# Docs-as-Code API

A stateless Go HTTP API that processes Markdown files, validates them, converts them to PDFs, and provides a status endpoint for tracking conversion jobs.

## Architecture

The service is built with clean architecture principles, using dependency injection to decouple business logic from infrastructure concerns.

### Key Components

- **HTTP Handler Layer** (`internal/http/handlers.go`): Manages HTTP requests and responses
- **Document Store** (`internal/store/store.go`): Interface for document job persistence
- **Converter** (`internal/converter/converter.go`): Interface for Markdown-to-PDF conversion
- **Main Entry Point** (`cmd/api/main.go`): Application bootstrap and dependency injection

### Dependency Injection Pattern

Dependencies are injected into the `Server` struct at initialization time:

```go
server := httphandlers.NewServer(documentStore, converter)
```

This approach enables:
- Easy testing with mock implementations
- Runtime flexibility to swap implementations
- Clear dependency declarations

## API Endpoints

### POST /v1/documents

Upload a Markdown file for PDF conversion.

**Request:**
- Method: `POST`
- Content-Type: `multipart/form-data`
- Body: Form field named `file` containing the Markdown file

**Response:**
- Status: `202 Accepted`
- Body:
  ```json
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending"
  }
  ```

**Error Responses:**
- `400 Bad Request`: Missing or invalid file
- `500 Internal Server Error`: Server-side processing error

### GET /v1/documents/{id}

Retrieve the status of a document conversion job.

**Request:**
- Method: `GET`
- Path Parameter: `id` - The job ID from the POST response

**Response:**
- Status: `200 OK`
- Body:
  ```json
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending"
  }
  ```

**Error Responses:**
- `400 Bad Request`: Missing job ID
- `404 Not Found`: Job does not exist
- `500 Internal Server Error`: Server-side retrieval error

## Running the Service

### Prerequisites

- Go 1.23 or later

### Installation

```bash
go mod download
go mod tidy
```

### Start the Server

```bash
go run cmd/api/main.go
```

The server will start on `http://localhost:8080`.

### Testing the API

**Create a document job:**
```bash
curl -X POST http://localhost:8080/v1/documents \
  -F "file=@example.md"
```

**Check job status:**
```bash
curl http://localhost:8080/v1/documents/{job-id}
```

## Current Implementation

### In-Memory Store

The `MemoryStore` provides a thread-safe, in-memory implementation of the `DocumentStore` interface suitable for development and testing.

**Features:**
- Generates unique job IDs using UUID v4
- Thread-safe operations with `sync.RWMutex`
- Stores file content in memory
- Validates file content (rejects empty files)

**Limitations:**
- Data is lost when the server restarts
- Not suitable for production without persistence
- Single-instance only (no horizontal scaling)

## Project Structure

```
.
├── cmd/
│   └── api/
│       └── main.go           # Application entry point
├── internal/
│   ├── converter/
│   │   └── converter.go      # PDF converter interface
│   ├── http/
│   │   └── handlers.go       # HTTP request handlers
│   └── store/
│       ├── store.go          # Store interface and Job struct
│       └── memory_store.go   # In-memory implementation
├── go.mod                    # Module definition
├── go.sum                    # Dependency lock file
└── README.md                 # This file
```

## Next Steps

1. **Implement PDF Converter**: Implement the `Converter` interface using a library like `chromedp` or `wkhtmltopdf`
2. **Add Persistence**: Replace `MemoryStore` with a database-backed implementation (PostgreSQL, MongoDB, etc.)
3. **Add Request Validation**: Implement Markdown validation and file type checking
4. **Add Middleware**: Implement request logging, error recovery, and rate limiting
5. **Add Tests**: Create unit and integration tests
6. **Add Configuration**: Support environment variables for port, logging level, etc.

## Dependencies

- `github.com/go-chi/chi/v5`: HTTP router and middleware library
- `github.com/google/uuid`: UUID generation for job IDs
