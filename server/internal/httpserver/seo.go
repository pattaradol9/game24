// SEO helpers: per-route HTML meta rewriting for crawlers that do not run
// JavaScript (link previews, some bots), plus dynamically generated
// robots.txt and sitemap.xml. The Vue SPA updates the same tags
// client-side on navigation (web/src/seo.js); this file covers the
// first HTML response so shared links always render a correct card.
package httpserver

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
)

const (
	defaultTitle       = "24 Game — รวมเลขให้ได้ 24!"
	defaultDescription = "เกมไพ่คณิตศาสตร์คลาสสิก จับไพ่ 4 ใบ ผสมด้วย + − × ÷ ให้ได้เลข 24 เล่นเดี่ยวเก็บ EXP ไต่ tier หรือสร้างห้องแข่งกับเพื่อนแบบเรียลไทม์ ฟรี ไม่ต้องสมัคร"
)

type seoMeta struct {
	Title       string
	Description string
	URL         string // absolute canonical URL of the route
	Image       string // absolute OG image URL
	Noindex     bool
}

// seoMetaForPath builds the meta for a client-side route, mirroring the
// per-route table in web/src/main.js. Titles stay Thai to match the
// site's primary language (<html lang="th">).
func seoMetaForPath(path, baseURL string) seoMeta {
	m := seoMeta{
		Title:       defaultTitle,
		Description: defaultDescription,
		Image:       baseURL + "/og-card.png",
		URL:         baseURL + path,
	}
	switch {
	case strings.HasPrefix(path, "/room/"):
		code := sanitizeRoomCode(strings.TrimPrefix(path, "/room/"))
		if code != "" {
			m.Title = "เข้าร่วมห้อง " + code + " · 24 Game"
			m.URL = baseURL + "/room/" + code
		} else {
			m.Title = "เข้าร่วมห้องแข่ง · 24 Game"
			m.URL = baseURL + "/room"
		}
		m.Description = "แข่งสดกับเพื่อน ใบไพ่เดียวกัน ผู้ที่ตอบถูกก่อนชนะรอบนั้น เข้าร่วมด้วยรหัสห้อง 6 หลัก"
		m.Noindex = true // private rooms must stay out of search indexes
	case strings.HasPrefix(path, "/admin"):
		m.Title = "Admin portal · 24 Game"
		m.Description = "Back-office portal for 24 Game administrators."
		m.Noindex = true
	case strings.HasPrefix(path, "/solo"):
		m.Title = "เล่นเดี่ยว — เก็บ EXP ไต่ Tier · 24 Game"
		m.Description = "เล่นเดี่ยว 4 โหมดความยาก (Jack, Queen, King, Ace) มีจับเวลา สตรีค และคำใบ้ เก็บ EXP สะสมเลเวลและ tier"
	case strings.HasPrefix(path, "/leaderboard"):
		m.Title = "จัดอันดับผู้เล่น · 24 Game"
		m.Description = "กระดานจัดอันดับรายโหมด ดูอันดับ EXP เลเวล และ tier ของผู้เล่นทั้งหมด"
	case strings.HasPrefix(path, "/privacy"):
		m.Title = "นโยบายความเป็นส่วนตัว · 24 Game"
		m.Description = "นโยบายความเป็นส่วนตัวของ 24 Game — ข้อมูลที่เก็บ การเข้ารหัส และสิทธิของผู้เล่น"
	case strings.HasPrefix(path, "/terms"):
		m.Title = "ข้อกำหนดการใช้งาน · 24 Game"
		m.Description = "ข้อกำหนดการใช้งาน 24 Game"
	}
	return m
}

// sanitizeRoomCode keeps only [A-Z0-9] and caps the length, so a crafted
// URL can never inject markup through the room code.
func sanitizeRoomCode(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	var b strings.Builder
	for _, r := range raw {
		isAlnum := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlnum {
			continue
		}
		if b.Len() >= 8 {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

var (
	seoTitleRe = regexp.MustCompile(`(?s)(<title>).*?(</title>)`)
	seoRules   = []struct {
		re  *regexp.Regexp
		get func(m seoMeta) string
	}{
		{regexp.MustCompile(`(?s)(<meta name="description" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Description }},
		{regexp.MustCompile(`(?s)(<meta name="robots" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string {
			if m.Noindex {
				return "noindex, noarchive"
			}
			return "index, follow"
		}},
		{regexp.MustCompile(`(?s)(<link rel="canonical" href=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.URL }},
		{regexp.MustCompile(`(?s)(<meta property="og:title" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Title }},
		{regexp.MustCompile(`(?s)(<meta property="og:description" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Description }},
		{regexp.MustCompile(`(?s)(<meta property="og:url" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.URL }},
		{regexp.MustCompile(`(?s)(<meta property="og:image" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Image }},
		{regexp.MustCompile(`(?s)(<meta name="twitter:title" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Title }},
		{regexp.MustCompile(`(?s)(<meta name="twitter:description" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Description }},
		{regexp.MustCompile(`(?s)(<meta name="twitter:image" content=").*?("[[:space:]]*/>)`), func(m seoMeta) string { return m.Image }},
	}
)

// renderIndex rewrites the static index.html meta tags for one route.
// Values are HTML-escaped, and "$" is doubled because regexp
// replacement strings treat it as a group reference.
func renderIndex(index []byte, m seoMeta) []byte {
	out := seoTitleRe.ReplaceAll(index, []byte("${1}"+escapeReplacement(m.Title)+"${2}"))
	for _, rule := range seoRules {
		out = rule.re.ReplaceAll(out, []byte("${1}"+escapeReplacement(rule.get(m))+"${2}"))
	}
	return out
}

func escapeReplacement(v string) string {
	return strings.ReplaceAll(html.EscapeString(v), "$", "$$")
}

// requestBaseURL returns the canonical origin for absolute URLs:
// PUBLIC_URL when configured, otherwise derived from the request's
// Host header (behind Cloud Run/LB the proxy forwards the real host).
func requestBaseURL(r *http.Request, configured string) string {
	if configured != "" {
		return configured
	}
	if r.Host == "" {
		return ""
	}
	scheme := "https"
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	return scheme + "://" + r.Host
}

// robotsHandler serves a generated robots.txt. Without a configured
// PUBLIC_URL the deployment has no known production origin, so indexing
// is blocked entirely (protects staging/preview URLs from being indexed).
func robotsHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		var b strings.Builder
		if baseURL == "" {
			b.WriteString("# No PUBLIC_URL configured: block indexing of this unknown origin.\n")
			b.WriteString("User-agent: *\nDisallow: /\n")
			_, _ = w.Write([]byte(b.String()))
			return
		}
		b.WriteString("User-agent: *\n")
		b.WriteString("Disallow: /admin\n")
		b.WriteString("Disallow: /room/\n")
		b.WriteString("Disallow: /api/\n")
		b.WriteString("\n")
		b.WriteString("Sitemap: " + baseURL + "/sitemap.xml\n")
		_, _ = w.Write([]byte(b.String()))
	}
}

var sitemapPaths = []struct {
	path     string
	priority string
}{
	{"/", "1.0"},
	{"/solo", "0.9"},
	{"/leaderboard", "0.7"},
	{"/privacy", "0.3"},
	{"/terms", "0.3"},
}

// sitemapHandler serves the fixed public routes with absolute URLs.
// Room links and /admin are private and deliberately excluded. Requires
// PUBLIC_URL: a sitemap must advertise one stable origin.
func sitemapHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if baseURL == "" {
			http.Error(w, "sitemap requires PUBLIC_URL to be configured", http.StatusNotFound)
			return
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
		for _, p := range sitemapPaths {
			fmt.Fprintf(&b, "  <url><loc>%s%s</loc><priority>%s</priority></url>\n",
				html.EscapeString(baseURL), p.path, p.priority)
		}
		b.WriteString("</urlset>\n")
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte(b.String()))
	}
}
