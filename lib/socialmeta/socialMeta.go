// Package socialmeta builds the Open Graph + Twitter Card <meta> tag block
// emitted on the homepage, pad page, and timeslider so URLs shared in chat
// apps (WhatsApp, Signal, Slack, ...) unfurl with a preview.
//
// Ported from upstream etherpad-lite (PR #7635, src/node/utils/socialMeta.ts).
//
// Security boundary: pad names from the URL are user-controlled. All values
// are HTML-escaped before interpolation to prevent reflected XSS via crafted
// pad IDs. og:url and og:image are built from settings.publicURL when set
// (operator-trusted); otherwise from the request's protocol + Host with
// strict Host validation so a crafted Host header cannot appear in og:url.
package socialmeta

import (
	"net/http"
	"regexp"
	"strings"
)

const socialDescriptionKey = "pad.social.description"

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

func escapeHTML(s string) string { return htmlEscaper.Replace(s) }

// Kind identifies which template the meta block is being rendered into.
// Affects the og:title composition.
type Kind string

const (
	KindHome       Kind = "home"
	KindPad        Kind = "pad"
	KindTimeslider Kind = "timeslider"
)

// Settings is the narrow shape socialmeta actually reads from the global
// Settings struct. Keeps the package decoupled from the full Settings surface.
type Settings struct {
	Title     string
	Favicon   string
	PublicURL string
}

// Locales maps a language tag (e.g. "en", "de", "de-AT") to its translation
// table for that language. socialmeta only reads the socialDescriptionKey
// entry; the rest of the table is ignored.
type Locales map[string]map[string]string

// RequestInfo bundles the per-request facts the renderer needs. The package
// avoids importing fiber/http directly so callers can adapt from any router.
type RequestInfo struct {
	// Scheme is "http" or "https". Anything else is normalised to "http".
	Scheme string
	// Host is the raw Host header (or X-Forwarded-Host if the router has
	// already resolved it). Will be strictly validated.
	Host string
	// Path is the request pathname (no query string).
	Path string
	// AcceptLanguage is the raw Accept-Language header for language
	// negotiation. May be empty.
	AcceptLanguage string
}

// FromHTTPRequest builds a RequestInfo from a net/http request. Fiber
// callers can build one directly from fiber.Ctx.
func FromHTTPRequest(r *http.Request) RequestInfo {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return RequestInfo{
		Scheme:         scheme,
		Host:           r.Host,
		Path:           r.URL.Path,
		AcceptLanguage: r.Header.Get("Accept-Language"),
	}
}

// Opts collects everything needed to render the meta block.
type Opts struct {
	Req            RequestInfo
	Settings       Settings
	AvailableLangs map[string]struct{}
	Locales        Locales
	Kind           Kind
	PadName        string // ignored for KindHome
}

// hostRe enforces a strict hostname[:port] format. Rejects CRLF injection,
// userinfo (user@host), wildcards, and any non-DNS character. The 255-byte
// hostname cap (plus optional port) is enforced separately.
var hostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]{0,253}[a-zA-Z0-9])?(:[0-9]{1,5})?$`)

func sanitizeHost(host string) string {
	if host == "" || len(host) > 255 {
		return ""
	}
	if !hostRe.MatchString(host) {
		return ""
	}
	return host
}

// publicURLRe accepts http(s)://host[:port] with no path, no userinfo, no
// trailing slash. Trailing slashes are stripped before matching.
var publicURLRe = regexp.MustCompile(`^(https?)://([^/?#]+)$`)

func sanitizePublicURL(raw string) string {
	if raw == "" {
		return ""
	}
	trimmed := strings.TrimRight(raw, "/")
	m := publicURLRe.FindStringSubmatch(trimmed)
	if m == nil {
		return ""
	}
	if sanitizeHost(m[2]) == "" {
		return ""
	}
	return strings.ToLower(m[1]) + "://" + m[2]
}

// buildAbsoluteURL returns scheme://host/pathname. Prefers a sanitised
// settings.publicURL; falls back to the request's scheme + validated Host;
// last-resort "localhost" if the Host header is unusable.
func buildAbsoluteURL(req RequestInfo, pathname, publicURL string) string {
	if trusted := sanitizePublicURL(publicURL); trusted != "" {
		return trusted + pathname
	}
	scheme := "http"
	if req.Scheme == "https" {
		scheme = "https"
	}
	host := sanitizeHost(req.Host)
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + pathname
}

func resolveImageURL(req RequestInfo, faviconSetting, publicURL string) string {
	if faviconSetting != "" {
		l := strings.ToLower(faviconSetting)
		if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
			return faviconSetting
		}
	}
	return buildAbsoluteURL(req, "/favicon.ico", publicURL)
}

func resolveDescription(locales Locales, renderLang string) string {
	if locales == nil {
		return ""
	}
	if m, ok := locales[renderLang]; ok {
		if v, ok := m[socialDescriptionKey]; ok && v != "" {
			return v
		}
	}
	primary := renderLang
	if i := strings.Index(renderLang, "-"); i >= 0 {
		primary = renderLang[:i]
	}
	if primary != renderLang {
		if m, ok := locales[primary]; ok {
			if v, ok := m[socialDescriptionKey]; ok && v != "" {
				return v
			}
		}
	}
	if m, ok := locales["en"]; ok {
		if v, ok := m[socialDescriptionKey]; ok {
			return v
		}
	}
	return ""
}

func toOgLocale(renderLang string) string {
	parts := strings.SplitN(renderLang, "-", 2)
	if len(parts) == 2 {
		return strings.ToLower(parts[0]) + "_" + strings.ToUpper(parts[1])
	}
	return strings.ToLower(parts[0])
}

// negotiateRenderLang picks the best language for the response. Very small
// subset of RFC 7231: scans Accept-Language tags in order and returns the
// first one that's in availableLangs. Falls back to "en".
func negotiateRenderLang(req RequestInfo, availableLangs map[string]struct{}) string {
	if req.AcceptLanguage == "" || len(availableLangs) == 0 {
		return "en"
	}
	for _, raw := range strings.Split(req.AcceptLanguage, ",") {
		tag := strings.TrimSpace(strings.SplitN(raw, ";", 2)[0])
		if tag == "" {
			continue
		}
		if _, ok := availableLangs[tag]; ok {
			return tag
		}
		// Try primary subtag.
		if i := strings.Index(tag, "-"); i > 0 {
			primary := tag[:i]
			if _, ok := availableLangs[primary]; ok {
				return primary
			}
		}
	}
	return "en"
}

// Render returns the HTML <meta> tag block (no surrounding tags, no
// trailing newline). Safe to drop into a templ template via @templ.Raw or
// equivalent escape-bypass mechanism.
func Render(o Opts) string {
	renderLang := negotiateRenderLang(o.Req, o.AvailableLangs)
	siteName := o.Settings.Title
	if siteName == "" {
		siteName = "Etherpad"
	}
	description := resolveDescription(o.Locales, renderLang)
	imageURL := resolveImageURL(o.Req, o.Settings.Favicon, o.Settings.PublicURL)
	imageAlt := siteName + " logo"

	title := siteName
	if o.PadName != "" {
		switch o.Kind {
		case KindPad:
			title = o.PadName + " | " + siteName
		case KindTimeslider:
			title = o.PadName + " (history) | " + siteName
		}
	}
	pathname := o.Req.Path
	if i := strings.Index(pathname, "?"); i >= 0 {
		pathname = pathname[:i]
	}
	if pathname == "" {
		pathname = "/"
	}
	canonical := buildAbsoluteURL(o.Req, pathname, o.Settings.PublicURL)

	var b strings.Builder
	tag := func(prop, value, attr string) {
		b.WriteString(`  <meta `)
		b.WriteString(attr)
		b.WriteString(`="`)
		b.WriteString(prop)
		b.WriteString(`" content="`)
		b.WriteString(escapeHTML(value))
		b.WriteString(`">` + "\n")
	}
	tag("og:type", "website", "property")
	tag("og:site_name", siteName, "property")
	tag("og:title", title, "property")
	tag("og:description", description, "property")
	tag("og:url", canonical, "property")
	tag("og:image", imageURL, "property")
	tag("og:image:alt", imageAlt, "property")
	tag("og:locale", toOgLocale(renderLang), "property")
	tag("twitter:card", "summary", "name")
	tag("twitter:title", title, "name")
	tag("twitter:description", description, "name")
	tag("twitter:image", imageURL, "name")
	tag("twitter:image:alt", imageAlt, "name")
	return strings.TrimRight(b.String(), "\n")
}
