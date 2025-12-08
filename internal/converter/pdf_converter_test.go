package converter

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPDFConverterMarkdownToPDF(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		minSize   int // Minimum PDF size in bytes
	}{
		{
			name:      "Simple markdown",
			input:     "# Hello\n\nThis is a test.",
			wantError: false,
			minSize:   100,
		},
		{
			name:      "Markdown with formatting",
			input:     "# Title\n\n**Bold** and *italic*",
			wantError: false,
			minSize:   100,
		},
		{
			name:      "Markdown with list",
			input:     "# List\n\n- Item 1\n- Item 2\n- Item 3",
			wantError: false,
			minSize:   100,
		},
		{
			name:      "Empty markdown",
			input:     "",
			wantError: false,
			minSize:   0,
		},
		{
			name:      "Markdown with code",
			input:     "# Code\n\n```\ncode here\n```",
			wantError: false,
			minSize:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewPDFConverter()
			input := strings.NewReader(tt.input)
			var output bytes.Buffer

			err := converter.MarkdownToPDF(context.Background(), input, &output)

			if (err != nil) != tt.wantError {
				t.Errorf("MarkdownToPDF error = %v, wantError %v", err, tt.wantError)
			}

			if !tt.wantError && output.Len() < tt.minSize {
				t.Errorf("PDF output size = %d, expected at least %d", output.Len(), tt.minSize)
			}

			// Verify PDF magic number
			if output.Len() > 0 {
				data := output.Bytes()
				if !bytes.HasPrefix(data, []byte("%PDF")) {
					t.Error("Output is not a valid PDF (missing %PDF header)")
				}
			}
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "<p>Hello</p>",
			expected: "Hello",
		},
		{
			input:    "<h1>Title</h1>",
			expected: "Title",
		},
		{
			input:    "<b>Bold</b> text",
			expected: "Bold text",
		},
		{
			input:    "No tags here",
			expected: "No tags here",
		},
		{
			input:    "&lt;tag&gt;",
			expected: "<tag>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripHTML(tt.input)
			if result != tt.expected {
				t.Errorf("stripHTML(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewPDFConverter(t *testing.T) {
	converter := NewPDFConverter()
	if converter == nil {
		t.Error("NewPDFConverter returned nil")
	}
}

func BenchmarkMarkdownToPDF(b *testing.B) {
	converter := NewPDFConverter()
	markdown := strings.Repeat("# Title\n\nThis is a paragraph with some content.\n\n", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := strings.NewReader(markdown)
		var output bytes.Buffer
		if err := converter.MarkdownToPDF(context.Background(), input, &output); err != nil {
			b.Fatalf("MarkdownToPDF failed: %v", err)
		}
	}
}
