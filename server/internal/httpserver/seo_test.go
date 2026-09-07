package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

const testIndex = `<!doctype html>
<html lang="th">
  <head>
    <title>24 Game — รวมเลขให้ได้ 24!</title>
    <meta name="description" content="default description" />
    <meta name="robots" content="index, follow" />
    <link rel="canonical" href="/" />
    <meta property="og:title" content="24 Game — รวมเลขให้ได้ 24!" />
    <meta property="og:url" content="/" />
    <meta property="og:image" content="/og-card.png" />
    <meta name="twitter:title" content="24 Game — รวมเลขให้ได้ 24!" />
  </head>
</html>`

func TestSeoMetaForPathHome(t *testing.T) {
	m := seoMetaForPath("/", "https://game.example")
	if m.Title != defaultTitle || m.Description != defaultDescription {
		t.Fatalf("home meta should use defaults, got title=%q", m.Title)
	}
	if m.URL != "https://game.example/" || m.Image != "https://game.example/og-card.png" {
		t.Fatalf("home URL/image wrong: %q %q", m.URL, m.Image)
	}
	if m.Noindex {
		t.Fatal("home must be indexable")
	}
}

func TestSeoMetaForPathRoom(t *testing.T) {
	m := seoMetaForPath("/room/482913", "https://game.example")
	if !m.Noindex {
		t.Fatal("room pages must be noindex")
	}
	if !strings.Contains(m.Title, "482913") {
		t.Fatalf("room title should contain the code, got %q", m.Title)
	}
	if m.URL != "https://game.example/room/482913" {
		t.Fatalf("room canonical wrong: %q", m.URL)
	}
}

func TestSanitizeRoomCode(t *testing.T) {
	cases := map[string]string{
		"482913":                    "482913",
		"ab-cd!ef":                  "ABCDEF",
		"  123456  ":                "123456",
		"<script>alert(1)</script>": "SCRIPTAL",
		"":                          "",
	}
	for in, want := range cases {
		if got := sanitizeRoomCode(in); got != want {
			t.Errorf("sanitizeRoomCode(%q) = %q, want %q", in, got, want)
		}
	}
	// longer than 8 chars -> truncated
	if got := sanitizeRoomCode("1234567890"); len(got) != 8 {
		t.Errorf("expected truncation to 8 chars, got %q", got)
	}
}

func TestRenderIndexRewritesAllTags(t *testing.T) {
	meta := seoMetaForPath("/room/482913", "https://game.example")
	out := string(renderIndex([]byte(testIndex), meta))

	checks := []string{
		"<title>เข้าร่วมห้อง 482913 · 24 Game</title>",
		`<meta name="robots" content="noindex, noarchive" />`,
		`<link rel="canonical" href="https://game.example/room/482913" />`,
		`<meta property="og:title" content="เข้าร่วมห้อง 482913 · 24 Game" />`,
		`<meta property="og:url" content="https://game.example/room/482913" />`,
		`<meta property="og:image" content="https://game.example/og-card.png" />`,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("rendered index missing %q\nrendered:\n%s", want, out)
		}
	}
	if strings.Contains(out, "default description") && meta.Description == "" {
		t.Error("stale default description survived")
	}
}

func TestRenderIndexEscapesMarkup(t *testing.T) {
	meta := seoMetaForPath(`/room/"><script>alert(1)</script>`, "https://game.example")
	out := string(renderIndex([]byte(testIndex), meta))
	if strings.Contains(out, "<script>") {
		t.Errorf("raw markup leaked into meta tags:\n%s", out)
	}
}

func TestRobotsHandlerEnabled(t *testing.T) {
	srv := httptest.NewRecorder()
	robotsHandler("https://game.example")(srv, httptest.NewRequest("GET", "/robots.txt", nil))
	body := srv.Body.String()
	for _, want := range []string{"Disallow: /admin", "Disallow: /room/", "Sitemap: https://game.example/sitemap.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt missing %q, got:\n%s", want, body)
		}
	}
}

func TestRobotsHandlerWithoutBaseURL(t *testing.T) {
	srv := httptest.NewRecorder()
	robotsHandler("")(srv, httptest.NewRequest("GET", "/robots.txt", nil))
	body := srv.Body.String()
	if !strings.Contains(body, "Disallow: /\n") || strings.Contains(body, "Sitemap:") {
		t.Errorf("robots.txt must block indexing entirely without PUBLIC_URL, got:\n%s", body)
	}
}

func TestSitemapHandler(t *testing.T) {
	srv := httptest.NewRecorder()
	sitemapHandler("https://game.example")(srv, httptest.NewRequest("GET", "/sitemap.xml", nil))
	body := srv.Body.String()
	for _, want := range []string{
		"<loc>https://game.example/</loc>",
		"<loc>https://game.example/solo</loc>",
		"<loc>https://game.example/leaderboard</loc>",
		"<loc>https://game.example/privacy</loc>",
		"<loc>https://game.example/terms</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap.xml missing %q, got:\n%s", want, body)
		}
	}
	if strings.Contains(body, "/room") || strings.Contains(body, "/admin") {
		t.Error("sitemap.xml must not list private routes")
	}
}

func TestSitemapHandlerWithoutBaseURL(t *testing.T) {
	srv := httptest.NewRecorder()
	sitemapHandler("")(srv, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if srv.Code != 404 {
		t.Errorf("sitemap without PUBLIC_URL should 404, got %d", srv.Code)
	}
}
