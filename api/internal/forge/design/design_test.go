package design

import (
	"strings"
	"testing"
)

func TestThemesLoaded(t *testing.T) {
	if len(Themes()) != 22 {
		t.Fatalf("themes = %d", len(Themes()))
	}
	if ThemeByID("nope").ID != DefaultTheme {
		t.Fatal("unknown theme must fall back")
	}
	if ThemeByID("midnight").Colors["background"] == "" {
		t.Fatal("expected parsed colors")
	}
	if !strings.Contains(ThemeByID("neon").fullCSS(), "--shadow-lg") {
		t.Fatal("base variables missing")
	}
}

func TestParsePlan(t *testing.T) {
	good := "Sure!\n```json\n{\"title\":\"Fit\",\"theme\":\"midnight\",\"screens\":[{\"id\":\"Home Page\",\"name\":\"Home\"},{\"id\":\"Home Page\",\"name\":\"\"},{\"id\":\"c\",\"name\":\"C\"}]}\n```"
	p, err := ParsePlan(good, 2, "")
	if err != nil || len(p.Screens) != 2 || p.Theme != "midnight" {
		t.Fatalf("%v %+v", err, p)
	}
	if p.Screens[0].ID != "home-page" || p.Screens[1].ID != "screen-2" || p.Screens[1].Name != "Screen 2" {
		t.Fatalf("ids: %+v", p.Screens)
	}
	if p, _ := ParsePlan(good, 4, "neon"); p.Theme != "neon" {
		t.Fatal("fixed theme must win")
	}
	if p, _ := ParsePlan(`{"theme":"bogus","screens":[{"name":"a"}]}`, 4, ""); p.Theme != DefaultTheme {
		t.Fatal("bogus theme must fall back")
	}
	for _, bad := range []string{"no json", `{"screens":[]}`, `{"screens": 3}`} {
		if _, err := ParsePlan(bad, 4, ""); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestCleanHTML(t *testing.T) {
	in := "Here you go:\n```html\n<div class=\"a\" onclick=\"x()\"><a href=\"javascript:alert(1)\">hi</a><script>bad()</script><img src=x onerror=alert(1)></div>\n```\nEnjoy"
	out, err := cleanHTML(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<script", "onclick", "onerror", "javascript:", "```", "Enjoy", "Here you go"} {
		if strings.Contains(out, bad) {
			t.Fatalf("%q survived: %s", bad, out)
		}
	}
	if !strings.HasPrefix(out, "<div") {
		t.Fatalf("got %s", out)
	}
	if _, err := cleanHTML("I cannot do that."); err == nil {
		t.Fatal("prose must be rejected")
	}
	body, _ := cleanHTML("<html><head><title>t</title></head><body><div>x</div></body></html>")
	if strings.Contains(body, "<head") || !strings.Contains(body, "<div>x</div>") {
		t.Fatalf("body extraction: %s", body)
	}
}

func TestCanvasRoundTrip(t *testing.T) {
	d := CanvasData{Title: "A </script><b>", Theme: "cyber", Device: "MOBILE", Width: 390, MinH: 844,
		Screens: []RenderedScreen{{ID: "s", Name: "S", HTML: `<div>x</script><script>alert(1)</script></div>`}}}
	doc, err := BuildCanvas(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	if strings.Contains(s, "x</script><script>alert(1)") || strings.Contains(s, "<b>") {
		t.Fatal("embedded data must not be able to close its script tag")
	}
	if !strings.Contains(s, `</script>`) {
		t.Fatal("expected angle brackets to be JSON-escaped in the payload")
	}
	if strings.Contains(s, "<title>A </script>") {
		t.Fatal("title must be escaped")
	}
	back, err := ParseCanvas(doc)
	if err != nil || back.Screens[0].HTML != d.Screens[0].HTML || back.Theme != "cyber" || back.Width != 390 {
		t.Fatalf("round trip: %v %+v", err, back)
	}
	if _, err := ParseCanvas([]byte("<html></html>")); err == nil {
		t.Fatal("non-canvas doc must fail")
	}
}

func TestDeviceFor(t *testing.T) {
	if DeviceFor("mobile").Width != 390 || DeviceFor("").Width != 1280 || DeviceFor("TABLET").Width != 820 {
		t.Fatal("device widths")
	}
}
