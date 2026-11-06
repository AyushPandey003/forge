package forge_test

import (
	"strings"
	"testing"

	"github.com/razorpay/slashdocs/api/internal/forge"
)

func TestMarkdownDocumentTitleMarkup(t *testing.T) {
	doc, err := forge.BuildMarkdownDoc(`Problem <statement> & review`, "# Problem\n\nEvidence")
	if err != nil {
		t.Fatal(err)
	}
	html := string(doc)
	if !strings.Contains(html, `<title>Problem &lt;statement&gt; &amp; review</title>`) {
		t.Fatal("document title must be escaped inside a valid title element")
	}
	if strings.Contains(html, "_Title_") {
		t.Fatal("placeholder title leaked into document")
	}
	if !strings.Contains(html, `<main class="doc"><h1`) {
		t.Fatal("document body missing")
	}
	if md, ok := forge.ExtractMarkdownSource(doc); !ok || md != "# Problem\n\nEvidence" {
		t.Fatal("Markdown source round-trip failed")
	}
}
