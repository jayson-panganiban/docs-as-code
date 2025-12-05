package store

import (
	"context"
	"io"
)

// Job represents a document conversion job.
type Job struct {
	ID     string
	Status string
}

// DocumentStore defines the interface for storing and retrieving document jobs.
type DocumentStore interface {
	CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error)
	GetDocumentJob(ctx context.Context, id string) (Job, error)
}
