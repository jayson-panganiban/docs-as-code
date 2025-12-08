# API Reference

## Base URL

```
http://localhost:8080
```

## Response Format

All responses are JSON-formatted. Success responses include `id` and `status` fields. Error responses include an `error` field.

### Success Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

### Error Response

```json
{
  "error": "file not provided or invalid"
}
```

## HTTP Status Codes

| Code | Meaning | Use Case |
|------|---------|----------|
| `200` | OK | Job found and status retrieved |
| `202` | Accepted | Job successfully created |
| `400` | Bad Request | Invalid request (missing file, malformed form) |
| `404` | Not Found | Job ID does not exist |
| `500` | Internal Server Error | Server-side processing failure |

## Endpoints

### POST /v1/documents

Create a new document conversion job from a Markdown file.

#### Request

**Headers:**
```
Content-Type: multipart/form-data
```

**Form Fields:**
- `file` (required): The Markdown file to convert

#### Response

**Success (202 Accepted):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

The `status` is always `"pending"` for newly created jobs.

**Error Examples:**

Missing file field (400):
```json
{
  "error": "file not provided or invalid"
}
```

Malformed form (400):
```json
{
  "error": "failed to parse form: multipart message too large"
}
```

Server error (500):
```json
{
  "error": "failed to create job: source file is empty"
}
```

#### cURL Example

```bash
curl -X POST http://localhost:8080/v1/documents \
  -F "file=@/path/to/document.md"
```

---

### GET /v1/documents/{id}

Retrieve the status of a document conversion job.

#### Path Parameters

| Name | Type | Description |
|------|------|-------------|
| `id` | string | The job ID returned from POST /v1/documents |

#### Response

**Success (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

**Possible Status Values:**
- `pending`: Job created, awaiting processing
- `processing`: Conversion in progress
- `completed`: Conversion successful
- `failed`: Conversion failed

**Error Examples:**

Missing job ID (400):
```json
{
  "error": "missing job id"
}
```

Job not found (404):
```json
{
  "error": "job not found"
}
```

Server error (500):
```json
{
  "error": "failed to retrieve job: database connection error"
}
```

#### cURL Example

```bash
curl http://localhost:8080/v1/documents/550e8400-e29b-41d4-a716-446655440000
```

---

## Job Lifecycle

A document conversion job follows this state machine:

```
┌─────────────┐
│   pending   │  ← Initial state after POST /v1/documents
└──────┬──────┘
       │
       ├──→ [processing]  ← Conversion in progress
       │         │
       │         ├──→ [completed]  ← Success
       │         │
       │         └──→ [failed]  ← Conversion error
       │
       └──→ [failed]  ← Immediate failure (validation, etc.)
```

## Rate Limiting

Currently, no rate limiting is implemented. Future versions may include request rate limiting.

## Authentication

Currently, no authentication is required. Future versions may include API key or OAuth2 authentication.

## File Size Limits

- Maximum file size: **10 MB**
- Files exceeding this limit will return a 400 Bad Request error

## Error Handling

The API uses standard HTTP status codes to indicate the outcome of a request:

- **2xx**: Success
- **4xx**: Client error (invalid request, missing data)
- **5xx**: Server error (processing failure)

All error responses include a descriptive `error` message to help diagnose the issue.

## Converter Implementation

The API now includes a full PDF converter implementation:

- **Markdown Parser**: Uses `goldmark` for RFC 7763 compliant parsing
- **PDF Generator**: Uses `gofpdf` for PDF creation
- **Features**: 
  - Supports headings, lists, code blocks, and basic formatting
  - Automatic page breaks
  - UTF-8 text support

All documents are converted from Markdown to PDF when the conversion job is processed.

## Examples

### Example 1: Create and Check Job Status

```bash
# Create a job
RESPONSE=$(curl -s -X POST http://localhost:8080/v1/documents \
  -F "file=@README.md")

# Extract job ID
JOB_ID=$(echo $RESPONSE | jq -r '.id')

# Check status
curl http://localhost:8080/v1/documents/$JOB_ID
```

### Example 2: Handle Errors

```bash
# Try to get a non-existent job
curl http://localhost:8080/v1/documents/invalid-id

# Response: 404 Not Found
# {
#   "error": "job not found"
# }
```

### Example 3: Upload Large File

```bash
# File larger than 10 MB will fail
curl -X POST http://localhost:8080/v1/documents \
  -F "file=@large-file.md"

# Response: 400 Bad Request
# {
#   "error": "failed to parse form: multipart message too large"
# }
```
