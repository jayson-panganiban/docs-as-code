You are an expert Go developer specializing in building robust, production-ready microservices. Your task is to generate the initial project scaffold for a "docs-as-code" API based on a provided architecture.

The service will be a stateless Go HTTP API that receives Markdown files, validates them, converts them to PDFs, and provides a status endpoint. The core architecture relies on dependency injection to handle storage, PDF conversion, and other concerns.

**Your Task:**

Generate the initial Go module setup and the code for `cmd/api/main.go` and placeholder stubs for the handlers and service interfaces.

**Requirements:**

1.  **Project Structure:** Create the following files:

    - `go.mod`
    - `cmd/api/main.go`
    - `internal/http/handlers.go`
    - `internal/converter/converter.go`
    - `internal/store/store.go`

2.  **`go.mod`:**

    - Initialize the module as `docs-as-code`.
    - Use Go version 1.23.
    - Include the `go-chi/chi/v5` router dependency.

3.  **`internal/converter/converter.go`:**

    - Define a `Converter` interface with a single method: `MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error`.

4.  **`internal/store/store.go`:**

    - Define a `DocumentStore` interface with placeholder methods for creating a document job and retrieving its status. For example:
      - `CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)` which returns a job ID.
      - `GetDocumentJob(ctx context.Context, id string) (Job, error)` which returns a `Job` struct.
    - Define a basic `Job` struct with fields like `ID` and `Status`.

5.  **`internal/http/handlers.go`:**

    - Create a `Server` struct that holds the service dependencies (e.g., `DocumentStore` and `Converter`).
    - Define the HTTP handler methods on this struct:
      - `handleCreateDocument(w http.ResponseWriter, r *http.Request)`
      - `handleGetDocumentStatus(w http.ResponseWriter, r *http.Request)`
    - The handlers should have the correct signatures but can contain placeholder logic (e.g., return a hardcoded JSON response).

6.  **`cmd/api/main.go`:**
    - Implement the main application entry point.
    - Set up a `chi` router.
    - Define the API routes:
      - `POST /v1/documents`
      - `GET /v1/documents/{id}`
    - Instantiate the `Server` from the `http` package.
    - Show where dependencies (like a database connection or the converter) would be initialized and injected into the `Server` struct. For now, you can pass `nil` or mock implementations.
    - Start the HTTP server on port `8080`.

**Instructions for Output:**

- Do **not** implement the full business logic for file storage, validation, or the actual PDF conversion.
- Focus only on creating the correct project structure, interfaces, dependency injection pattern, and router setup.
- Provide the complete, runnable code for each file.
- Enclose the code for each file in a separate, clearly labeled markdown block (e.g., "`cmd/api/main.go`").
