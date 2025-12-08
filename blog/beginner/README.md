# Building Your First Go Microservice - Beginner Series

## Welcome!

You've decided to learn Go by building a real microservice. That's the best way to learn—hands-on, with a goal.

This beginner-friendly series walks you through every step, explaining not just the *what* but the *why*. No Go knowledge required beyond understanding functions and packages.

## The Project

We're building a **docs-as-code API**: a service that accepts files, creates processing jobs, and lets you check the status. This pattern appears in real production systems everywhere.

By the end of this series, you'll have built:
- ✅ A working HTTP API
- ✅ Thread-safe data storage
- ✅ Comprehensive tests
- ✅ Production-grade error handling
- ✅ Graceful shutdown
- ✅ Environment-based configuration
- ✅ Health checks

## Series Parts

### Part 1: Getting Started
**Status**: ✅ Complete | **Difficulty**: Beginner

Learn the basics of structuring a Go project and building your first API.

**Topics**:
- Project structure (cmd/, internal/)
- Interfaces and why they matter
- Dependency injection patterns
- Writing your first HTTP handler
- Creating an in-memory store
- Wiring everything together

**What you build**: A working API that accepts files and stores jobs

📖 [Read Part 1: Getting Started](./01-getting-started.md)

---

### Part 2: Threads, Tests & JSON
**Status**: ✅ Complete | **Difficulty**: Beginner+

Make your API safe for concurrent requests and learn how to test it.

**Topics**:
- Understanding goroutines and concurrency
- Thread safety with mutexes
- RWMutex for read-heavy operations
- Writing unit tests
- Table-driven tests
- Testing with mocks
- JSON responses and error handling
- HTTP status codes

**What you build**: Thread-safe API with comprehensive test coverage

📖 [Read Part 2: Threads, Tests & JSON](./02-threads-tests-json.md)

---

### Part 3: PDF Conversion & Production Features
**Status**: ✅ Complete | **Difficulty**: Intermediate

Add logging, implement the converter, and make your API production-ready.

**Topics**:
- Implementing converter interface
- Structured logging (JSON logs)
- Graceful shutdown
- Environment variables
- Health checks
- Custom error types
- Integration testing
- Production best practices

**What you build**: Production-grade API ready to deploy

📖 [Read Part 3: PDF Conversion & Production Features](./03-pdf-conversion.md)

---

## Learning Paths

### Path 1: Understand the Basics
1. Read Part 1
2. Code along with Part 1
3. Try the exercises

**Result**: You understand Go project structure and HTTP handlers.

### Path 2: Build a Working API
1. Complete Path 1
2. Read Part 2
3. Add tests to your code
4. Try the exercises

**Result**: You have a tested, thread-safe API.

### Path 3: Production Ready
1. Complete Path 1 & 2
2. Read Part 3
3. Add logging and graceful shutdown
4. Deploy and test locally

**Result**: A real, production-grade microservice.

## Key Concepts by Topic

| Concept | Part |
|---------|------|
| Project Structure | 1 |
| Interfaces | 1 |
| Dependency Injection | 1 |
| HTTP Handlers | 1 |
| Goroutines | 2 |
| Mutexes & RWMutex | 2 |
| Unit Testing | 2 |
| Mocking | 2 |
| JSON | 2 |
| Logging | 3 |
| Graceful Shutdown | 3 |
| Configuration | 3 |
| Health Checks | 3 |
| Integration Testing | 3 |
| Production Practices | 3 |

## Code Snippets You'll Learn

### The Interface Pattern
```go
type DocumentStore interface {
    CreateJob(ctx context.Context, file io.Reader) (string, error)
    GetJob(ctx context.Context, id string) (Job, error)
}
```

### Dependency Injection
```go
func NewServer(store DocumentStore) *Server {
    return &Server{store: store}
}
```

### Thread Safety
```go
ms.mu.RLock()
defer ms.mu.RUnlock()
job := ms.jobs[id]  // Safe read
```

### Error Handling
```go
job, err := store.GetJob(ctx, id)
if err != nil {
    return err  // Propagate error up
}
```

### Testing
```go
func TestCreateJob(t *testing.T) {
    store := NewMemoryStore()
    id, err := store.CreateJob(ctx, file)
    if err != nil {
        t.Fatal(err)
    }
}
```

### JSON Response
```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(Response{ID: id, Status: "pending"})
```

### Graceful Shutdown
```go
signal.Notify(sigChan, syscall.SIGINT)
<-sigChan
server.Shutdown(context.Background())
```

## Prerequisites

**Must Know**:
- Basic Go syntax (functions, variables)
- Go packages and imports
- How to run Go programs (`go run`)

**Nice to Have**:
- Basic HTTP concepts
- What a JSON is
- Command line basics (curl, terminals)

**Don't Need**:
- Advanced Go knowledge
- Database experience
- Microservices experience
- Concurrency experience (we teach it!)

## Setup

Before you start, install Go:

```bash
# Check installation
go version

# Should output something like: go version go1.21.0 linux/amd64
```

Then clone the project:

```bash
git clone <repo-url>
cd docs-as-code
```

And follow along with Part 1!

## Running the Examples

Each part includes runnable code. To try it:

```bash
# Part 1
cd docs-as-code
go run cmd/api/main.go

# In another terminal
curl http://localhost:8080/health

# Stop with Ctrl+C
```

## Common Questions

### Q: Do I need to install dependencies?
A: Go will download them automatically when you first run `go mod tidy` or `go run`.

### Q: How long will this take?
A: It varies by your pace and whether you do exercises. Read and practice at your own speed.

### Q: Can I skip parts?
A: Not really. Part 2 builds on Part 1. Part 3 assumes you did Parts 1-2.

### Q: Can I use this in production?
A: The patterns? Yes! They're production-grade. The code? It's a starting point. You'd add database, real PDF conversion, monitoring, etc.

### Q: I'm stuck on an exercise
A: Check the answer in the next part's code. Or ask in the comments!

### Q: Why MemoryStore instead of a database?
A: Start simple. MemoryStore teaches you the patterns. Add PostgreSQL later when you need persistence.

### Q: Why not use a framework like Gin?
A: We use chi (a minimal router). It teaches you HTTP fundamentals. Frameworks hide too much for learning.

## Troubleshooting

### "command not found: go"
```bash
# Check Go is installed
which go

# If not installed, download from golang.org
```

### "module not found" errors
```bash
# Download dependencies
go mod download
go mod tidy
```

### "Address already in use"
```bash
# Another process is using port 8080
# Kill it:
lsof -i :8080        # Find process
kill -9 <PID>        # Kill it

# Or use a different port:
PORT=9000 go run cmd/api/main.go
```

### Tests failing
```bash
# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...
```

## Next Steps After This Series

Once you finish all 3 parts, consider:

1. **Add real features**:
   - Database (PostgreSQL/MongoDB)
   - Actual PDF conversion
   - File validation
   - Email notifications

2. **Deploy**:
   - Containerize with Docker
   - Deploy to Kubernetes
   - Set up CI/CD

3. **Monitor**:
   - Add Prometheus metrics
   - Set up alerting
   - Add distributed tracing

4. **Scale**:
   - Add caching (Redis)
   - Implement job queue (RabbitMQ)
   - Multi-instance load balancing

5. **Deepen Knowledge**:
   - Read "Effective Go"
   - Study concurrency patterns
   - Learn about microservices architecture

## Learning Resources

### Official Go Resources
- [Go Tour](https://go.dev/tour/welcome/1) - Interactive Go basics
- [Effective Go](https://golang.org/doc/effective_go) - Go best practices
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### Related Concepts
- [HTTP Protocol](https://developer.mozilla.org/en-US/docs/Web/HTTP) - HTTP basics
- [REST APIs](https://restfulapi.net/) - API design
- [JSON](https://www.json.org/) - Data format

### Advanced Topics (for after)
- **Concurrency**: "Concurrency in Go" by Katherine Cox-Buday
- **Architecture**: "Clean Architecture" by Robert C. Martin
- **Microservices**: "Building Microservices" by Sam Newman

## FAQ

### Q: Can I contribute to this series?
A: Yes! Found a typo? Better explanation? Open an issue or pull request.

### Q: Can I translate this?
A: Please! Let us know so we can link to your translation.

### Q: Can I use this for teaching?
A: Absolutely! Feel free to share with students. We'd love to hear about it.

### Q: Will there be more parts?
A: Yes! Part 4 (Database), Part 5 (Docker), Part 6 (Kubernetes) are in planning.

## Author Notes

This series came from years of teaching Go to beginners. The most common problem: jumping to advanced patterns before understanding fundamentals.

That's why we start with simple interfaces, explicit dependency injection, and concrete examples. We deliberately avoid magic and frameworks that hide the details.

By the end, you'll understand not just Go, but *why* certain patterns matter. That understanding will serve you in any language.

## Community

Have questions? Share your solutions?

- **Comments**: Leave questions on each part
- **GitHub Issues**: Report problems
- **Show & Tell**: Share your projects
- **Discussions**: Suggest improvements

## Getting Started

Ready to build your first microservice?

**👉 Start with [Part 1: Getting Started](./01-getting-started.md)**

---

*Welcome to Go! You picked a great language and a great way to learn it.*

**Happy coding! 🚀**
