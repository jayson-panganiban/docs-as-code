package converter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/phpdave11/gofpdf"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// PDFConverter implements Converter for Markdown to PDF conversion.
type PDFConverter struct{}

// NewPDFConverter creates a new PDF converter instance.
func NewPDFConverter() *PDFConverter {
	return &PDFConverter{}
}

// MarkdownToPDF converts Markdown content to PDF format.
func (pc *PDFConverter) MarkdownToPDF(ctx context.Context, in io.Reader, out io.Writer) error {
	// Read the Markdown content
	content, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("failed to read markdown input: %w", err)
	}

	// Parse Markdown to HTML
	html, err := pc.markdownToHTML(string(content))
	if err != nil {
		return fmt.Errorf("failed to convert markdown to html: %w", err)
	}

	// Convert HTML to PDF
	if err := pc.htmlToPDF(html, out); err != nil {
		return fmt.Errorf("failed to convert html to pdf: %w", err)
	}

	return nil
}

// markdownToHTML converts Markdown content to HTML.
func (pc *PDFConverter) markdownToHTML(markdown string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
	)

	var buf strings.Builder
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("markdown parsing failed: %w", err)
	}

	return buf.String(), nil
}

// htmlToPDF converts HTML content to PDF.
func (pc *PDFConverter) htmlToPDF(html string, out io.Writer) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)

	// Parse HTML and add to PDF
	// Simple implementation: split by lines and add text
	scanner := bufio.NewScanner(strings.NewReader(html))
	lineHeight := 10.0
	yPos := 20.0
	pageHeight := 297.0 // A4 height in mm
	margin := 10.0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			yPos += lineHeight * 0.5
			continue
		}

		// Remove HTML tags for display (simple approach)
		text := stripHTML(line)
		if text == "" {
			continue
		}

		// Check if we need a new page
		if yPos > pageHeight-margin {
			pdf.AddPage()
			yPos = margin
		}

		// Handle headings with larger font
		if strings.HasPrefix(line, "<h1") {
			pdf.SetFont("Arial", "B", 16)
			pdf.MultiCell(0, lineHeight, stripHTML(line), "", "", false)
			pdf.SetFont("Arial", "", 12)
		} else if strings.HasPrefix(line, "<h2") {
			pdf.SetFont("Arial", "B", 14)
			pdf.MultiCell(0, lineHeight, stripHTML(line), "", "", false)
			pdf.SetFont("Arial", "", 12)
		} else if strings.HasPrefix(line, "<h3") {
			pdf.SetFont("Arial", "B", 12)
			pdf.MultiCell(0, lineHeight, stripHTML(line), "", "", false)
			pdf.SetFont("Arial", "", 12)
		} else {
			pdf.MultiCell(0, lineHeight, text, "", "", false)
		}

		yPos = pdf.GetY() + lineHeight
	}

	// Write PDF to output
	return pdf.Output(out)
}

// stripHTML removes HTML tags from a string.
func stripHTML(html string) string {
	var result strings.Builder
	inTag := false

	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}

	// Decode HTML entities (basic)
	text := strings.ReplaceAll(result.String(), "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	return strings.TrimSpace(text)
}
