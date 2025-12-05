# AGENTS.md - Project Standards & Commands

This document serves as ground truth for commands, style, structure, and workflows for the docs-as-code project.

## Project Overview

**Name**: docs-as-code  
**Language**: Go 1.23  
**Type**: HTTP Microservice  
**Purpose**: Accept Markdown files, create conversion jobs, provide status tracking

**Repository**: https://github.com/jayson-panganiban/docs-as-code

## Directory Structure

```
docs-as-code/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── internal/
│   ├── converter/
│   │   └── converter.go            # Converter interface
│   ├── http/
│   │   └── handlers.go             # HTTP handlers
│   └── store/
│       ├── store.go                # Store interface & Job struct
│       └── memory_store.go         # In-memory implementation
├── docs/
│   ├── API.md                      # API reference
│   ├── ARCHITECTURE.md             # System design
│   └── IMPLEMENTATION.md           # Implementation details
├── blog/
│   ├── INDEX.md                    # Master blog index
│   ├── README.md                   # Advanced series index
│   ├── 01-initial-scaffold.md
│   ├── 02-design-patterns.md
│   ├── 03-implementation-walkthrough.md
│   └── beginner/
│       ├── README.md               # Beginner series index
│       ├── 01-getting-started.md
│       ├── 02-threads-tests-json.md
│       └── 03-pdf-conversion.md
├── go.mod                          # Module definition
├── go.sum                          # Dependency lock
├── README.md                       # Project overview
├── BLOG_SUMMARY.md                 # Content summary
├── PROMPT.md                       # Original requirements
└── AGENTS.md                       # This file
```

## Go Conventions

### Package Organization

- **cmd/**: Executable applications
  - One `main()` function per application
  - Minimal logic (bootstrap and wire dependencies)
  
- **internal/**: Private packages (cannot be imported externally)
  - Group by feature/concern
  - Each package has a clear responsibility
  - No circular dependencies

### Naming

- **Packages**: lowercase, short, descriptive
  - ✓ `http`, `store`, `converter`
  - ✗ `HTTP_Handler`, `StorageLayer`

- **Interfaces**: descriptive of behavior
  - ✓ `DocumentStore`, `Converter`
  - ✗ `Handler`, `Interface`

- **Constructors**: `NewTypeName()` returns `*TypeName`
  - ✓ `NewMemoryStore() *MemoryStore`
  - ✗ `CreateStore()`, `MakeMemoryStore()`

- **Methods**: descriptive verb phrases
  - ✓ `CreateDocumentJob()`, `GetDocumentJob()`
  - ✗ `Create()`, `Get()` (ambiguous)

### File Naming

- Test files: `*_test.go`
- Implementation files: descriptive noun
  - ✓ `memory_store.go`, `handlers.go`
  - ✗ `impl.go`, `handler_impl.go`

## Code Style

### General Principles

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `go fmt` for formatting (automatic via `go run`)
- Keep functions small and focused
- Interfaces > concrete types
- Composition > inheritance
- Explicit > implicit

### Error Handling

**Always check errors explicitly:**

```go
// ✓ GOOD
err := someOperation()
if err != nil {
    return fmt.Errorf("failed to do thing: %w", err)
}

// ✗ BAD
_ = someOperation()
result := doAnotherThing()  // Assumes first succeeded!
```

**Use error wrapping with %w:**

```go
// ✓ GOOD: Preserves original error chain
return fmt.Errorf("context: %w", originalError)

// ✗ BAD: Loses error type information
return fmt.Errorf("context: %v", originalError)
```

### Dependency Injection

**Always use constructor injection:**

```go
// ✓ GOOD
type Server struct {
    store store.DocumentStore
}

func NewServer(s store.DocumentStore) *Server {
    return &Server{store: s}
}

// ✗ BAD: Global state or service locator
var globalStore store.DocumentStore
func NewServer() *Server {
    return &Server{store: globalStore}
}
```

### Thread Safety

**Use defer with locks:**

```go
// ✓ GOOD: Guaranteed unlock
ms.mu.RLock()
defer ms.mu.RUnlock()
// ... operations

// ✗ BAD: Forget to unlock in early return
ms.mu.Lock()
if condition {
    return nil  // Lock never released!
}
ms.mu.Unlock()
```

**Choose appropriate lock type:**

- `sync.RWMutex` for read-heavy operations
- `sync.Mutex` for general protection
- `sync.Once` for one-time initialization

### Context Usage

**Always pass context through call stack:**

```go
// ✓ GOOD
func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
    err := s.store.CreateJob(r.Context(), file)
    // Context flows through
}

// ✗ BAD: Using context.Background() inappropriately
err := s.store.CreateJob(context.Background(), file)
// Ignores request cancellation/timeout
```

## Common Commands

### Development

```bash
# Run the server
go run cmd/api/main.go

# Run with environment variable
PORT=9000 go run cmd/api/main.go

# Build binary
go build -o bin/api cmd/api/main.go

# Run binary
./bin/api
```

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -v -run TestCreateJob ./internal/store

# Run with race detection
go test -race ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Dependencies

```bash
# Download dependencies
go mod download

# Tidy up go.mod and go.sum
go mod tidy

# Update specific dependency
go get -u github.com/go-chi/chi/v5

# View module graph
go mod graph

# Check for vulnerabilities
go list -json -m all | nancy sleuth
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter (requires golangci-lint)
golangci-lint run ./...

# Check for common issues
go vet ./...

# View TODO comments
grep -r "TODO" internal/
```

### Git Workflow

```bash
# View commit history
git log --oneline

# View changes
git status
git diff

# Stage changes
git add .

# Commit with message
git commit -m "Description of changes"

# Push to GitHub
git push origin main

# Pull latest changes
git pull origin main

# Create feature branch
git checkout -b feature/my-feature

# Switch branch
git checkout branch-name

# Delete branch
git branch -d feature/my-feature
```

## Testing Standards

### Test File Location

- Test files in same package as code being tested
- File naming: `*_test.go`

### Test Structure (AAA Pattern)

```go
func TestFeatureName(t *testing.T) {
    // Arrange: Set up test data
    store := NewMemoryStore()
    
    // Act: Execute the code being tested
    jobID, err := store.CreateJob(context.Background(), file)
    
    // Assert: Check the results
    if err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }
    if jobID == "" {
        t.Fatal("Expected jobID, got empty string")
    }
}
```

### Table-Driven Tests

```go
func TestScenarios(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        wantError bool
    }{
        {"Valid", "content", false},
        {"Empty", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := someFunc(tt.input)
            if (err != nil) != tt.wantError {
                t.Errorf("unexpected error")
            }
        })
    }
}
```

### Mocks for Testing

```go
// Define mock that implements interface
type MockStore struct {
    CreateFunc func(ctx context.Context, file io.Reader) (string, error)
}

func (m *MockStore) CreateJob(ctx context.Context, file io.Reader) (string, error) {
    return m.CreateFunc(ctx, file)
}

// Use in test
func TestHandler(t *testing.T) {
    mock := &MockStore{
        CreateFunc: func(ctx context.Context, file io.Reader) (string, error) {
            return "test-id", nil
        },
    }
    // Test with mock
}
```

## Documentation Standards

### README Files

Each section should have a README.md explaining:
- Purpose of the section
- How to use it
- Key files and their roles
- Example usage

### Code Comments

- Explain "why", not "what"
- Document exported functions and types
- Use `// Comment` format (not `/* */`)

```go
// ✓ GOOD: Explains purpose
// DocumentStore manages document conversion jobs
type DocumentStore interface {
    CreateJob(ctx context.Context, file io.Reader) (string, error)
}

// ✗ BAD: Just repeats code
// Create a job
func (s *Store) CreateJob(ctx context.Context, file io.Reader) (string, error) {
```

### API Documentation

- Document in docs/API.md
- Include request/response examples
- Document all status codes
- Document error responses

### Architecture Documentation

- Document in docs/ARCHITECTURE.md
- Include diagrams
- Explain design decisions
- Document trade-offs

## Blog Content Standards

### Blog Post Structure

1. **Introduction**: Hook and overview
2. **Problem Statement**: What are we solving?
3. **Solution**: Step-by-step explanation
4. **Code Examples**: Working code from actual project
5. **Key Takeaways**: Summary of lessons
6. **Resources**: Links for further learning

### Code in Blog Posts

- Extract from actual working code
- Include context and explanation
- Show both ✓ good and ✗ bad patterns
- Use comments to explain non-obvious parts

### Blog Categories

- **Beginner**: Explain from first principles, lots of examples
- **Advanced**: Focus on patterns, rationale, trade-offs

## HTTP API Standards

### Endpoint Design

```
POST   /v1/documents           # Create resource
GET    /v1/documents/{id}      # Get resource
GET    /health                 # Health check
```

### Response Format

All responses are JSON:

```json
{
  "id": "resource-id",
  "status": "state",
  "error": "optional error message"
}
```

### Status Codes

- `200 OK` - Request succeeded, resource returned
- `202 Accepted` - Async request accepted, processing
- `400 Bad Request` - Client error (invalid input)
- `404 Not Found` - Resource doesn't exist
- `500 Internal Server Error` - Server error

### Error Responses

```json
{
  "error": "descriptive error message"
}
```

## Project Conventions

### Feature Development

1. **Create branch**: `git checkout -b feature/name`
2. **Implement feature**: Code in feature branch
3. **Test thoroughly**: Unit + integration tests
4. **Document**: Update relevant docs
5. **Commit**: Clear, descriptive commit messages
6. **Push**: `git push origin feature/name`
7. **Merge**: Merge to main via PR or direct push

### Commit Messages

**Format**: `<type>: <description>`

```
feature: add health check endpoint
fix: handle empty file validation
docs: update API reference
refactor: simplify error handling
test: add concurrent access tests
```

### Version Control

- Use `main` as default branch
- Tag releases: `git tag v1.0.0`
- Keep commits atomic (one logical change per commit)
- Use descriptive commit messages

## Dependencies

### Go Modules

**Core Dependencies**:
- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/google/uuid` - UUID generation

**Testing Dependencies** (when added):
- Standard library `testing` package preferred
- External test frameworks only if necessary

### Adding Dependencies

```bash
# Add new dependency
go get github.com/package/name

# Add specific version
go get github.com/package/name@v1.2.3

# Update dependency
go get -u github.com/package/name

# Remove unused dependencies
go mod tidy
```

## Performance Considerations

### Benchmarking

```bash
# Run benchmarks
go test -bench=. ./...

# Compare benchmarks
go test -bench=. -benchmem ./...
```

### Profiling (when needed)

```go
import "runtime/pprof"

// CPU profiling
f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

// Memory profiling
f, _ := os.Create("mem.prof")
pprof.WriteHeapProfile(f)
```

## Security Considerations

### Input Validation

- Always validate input
- Check file sizes
- Validate content types
- Sanitize error messages (don't leak sensitive info)

### Secrets Management

- Use environment variables for secrets
- Never commit secrets to version control
- Document required environment variables

## Production Deployment

### Environment Variables

```bash
PORT              # Server port (default: 8080)
LOG_LEVEL         # Logging level (default: info)
STORE_BACKEND     # Storage backend (default: memory)
DATABASE_URL      # Database connection string (if using DB)
```

### Health Checks

- Implement `/health` endpoint
- Return `200 OK` when healthy
- Return `503 Service Unavailable` when unhealthy

### Logging

- Use structured logging (JSON format)
- Include context (request ID, user ID, etc.)
- Log errors with stack traces
- Don't log sensitive data

### Graceful Shutdown

- Listen for SIGINT/SIGTERM
- Stop accepting new connections
- Wait for in-flight requests to complete
- Close resources properly

## Troubleshooting Commands

### Common Issues

```bash
# Port already in use
lsof -i :8080
kill -9 <PID>

# Module not found
go mod download
go mod tidy

# Race conditions
go test -race ./...

# Test verbose output
go test -v ./...

# Check Go version
go version

# View module info
go list -m all
```

## Continuous Integration

### Pre-commit Checks

```bash
# Before committing, run:
go fmt ./...
go vet ./...
go test -race ./...
```

### CI Pipeline (when set up)

1. Run tests with race detection
2. Run linters
3. Generate coverage report
4. Build binary
5. Push to Docker registry (when implemented)

## Documentation Updates

When making changes:

1. Update relevant code comments
2. Update docs/ files if architecture changes
3. Update BLOG_SUMMARY.md if adding blog posts
4. Update README.md if changing project structure
5. Commit documentation changes with code changes

## Adding New Features

### Checklist

- [ ] Create interface (if needed)
- [ ] Write tests first (TDD preferred)
- [ ] Implement feature
- [ ] Test thread safety (if applicable)
- [ ] Update docs
- [ ] Add comments to code
- [ ] Run `go fmt` and `go vet`
- [ ] Run all tests including `-race` flag
- [ ] Update git history (squash if needed)
- [ ] Push to GitHub

## Resources

### Official Go Resources
- [Go Documentation](https://golang.org/doc)
- [Effective Go](https://golang.org/doc/effective_go)
- [Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### Project Resources
- [API Documentation](./docs/API.md)
- [Architecture Guide](./docs/ARCHITECTURE.md)
- [Implementation Details](./docs/IMPLEMENTATION.md)
- [Blog Series](./blog/INDEX.md)

### Tools Used
- Go 1.23
- chi/v5 (HTTP router)
- Google UUID (ID generation)

## Contact & Questions

For questions about this project:
- Check the blog series in `/blog`
- Review the docs in `/docs`
- Check the code in `/internal`
- Review PROMPT.md for original requirements

---

**Last Updated**: December 2024  
**Maintained By**: Project Contributors  
**Version**: 1.0
