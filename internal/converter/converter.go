package converter

import (
	"context"
	"io"
)

// Converter defines the interface for converting Markdown to PDF.
type Converter interface {
	MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}
