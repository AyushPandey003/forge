package forge

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// Authors are team members; raw HTML lets them embed images/diagrams.
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// MarkdownToHTML renders GitHub-flavoured Markdown.
func MarkdownToHTML(src string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// BuildMarkdownDoc renders Markdown into a complete Forge document.
func BuildMarkdownDoc(title, src string) ([]byte, error) {
	body, err := MarkdownToHTML(src)
	if err != nil {
		return nil, err
	}
	return RenderMarkdownDoc(title, body, src), nil
}
