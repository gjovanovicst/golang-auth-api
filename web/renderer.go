package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin/render"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed templates
var templateFS embed.FS

// TemplateData is the standard data structure passed to all templates.
// Handlers populate specific fields; the renderer ensures defaults.
type TemplateData struct {
	// Page metadata
	ActivePage string

	// Auth context (set by middleware)
	AdminUsername string
	AdminID       string

	// CSRF token (set by CSRF middleware)
	CSRFToken string

	// Flash messages
	FlashSuccess string
	FlashError   string

	// Login-specific fields
	Error    string // Login error message
	Username string // Pre-filled username on login error
	Redirect string // Post-login redirect URL

	// 2FA-specific fields
	TempToken   string // Temporary token for 2FA login verification
	TwoFAMethod string // "totp" or "email" — which 2FA method is required

	// Page-specific data (each page can put arbitrary data here)
	Data interface{}
}

// Renderer implements gin's render.HTMLRender interface using embedded templates.
type Renderer struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// NewRenderer creates a Renderer by parsing all embedded templates.
// Layout templates are combined with each page template so that
// {{template "base" .}} works from page templates.
func NewRenderer() (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template),
		funcMap:   defaultFuncMap(),
	}

	if err := r.parseTemplates(); err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return r, nil
}

// Instance returns a render.Render for a specific template name and data.
// This satisfies the render.HTMLRender interface.
func (r *Renderer) Instance(name string, data interface{}) render.Render {
	tmpl, ok := r.templates[name]
	if !ok {
		// Fallback: return an error render
		return &HTMLRender{
			Template: nil,
			Name:     name,
			Data:     data,
		}
	}
	return &HTMLRender{
		Template: tmpl,
		Name:     name,
		Data:     data,
	}
}

// parseTemplates reads layout and page templates from the embedded FS.
// Each page template is cloned from the layout set so it can use {{template "base" .}}.
func (r *Renderer) parseTemplates() error {
	// Parse all layout files
	layoutFiles, err := fs.Glob(templateFS, "templates/layouts/*.tmpl")
	if err != nil {
		return fmt.Errorf("failed to glob layouts: %w", err)
	}

	// Parse all partial files
	partialFiles, err := fs.Glob(templateFS, "templates/partials/*.tmpl")
	if err != nil {
		return fmt.Errorf("failed to glob partials: %w", err)
	}

	// Combine layouts + partials as the base template set
	baseFiles := append(layoutFiles, partialFiles...)

	// Parse each page template individually, combined with layouts + partials
	pageFiles, err := fs.Glob(templateFS, "templates/pages/*.tmpl")
	if err != nil {
		return fmt.Errorf("failed to glob pages: %w", err)
	}

	for _, pageFile := range pageFiles {
		// Extract the template name from the filename: "templates/pages/login.tmpl" -> "login"
		name := strings.TrimPrefix(pageFile, "templates/pages/")
		name = strings.TrimSuffix(name, ".tmpl")

		// Create a new template set with functions, parse base files + this page file
		files := append([]string{pageFile}, baseFiles...)
		tmpl, err := template.New(name).Funcs(r.funcMap).ParseFS(templateFS, files...)
		if err != nil {
			return fmt.Errorf("failed to parse template %q: %w", name, err)
		}

		r.templates[name] = tmpl
	}

	// Register partials as standalone templates for HTMX fragment responses.
	// All partial files are parsed together so partials can reference each other.
	// They are rendered without the base layout — just the partial's define block.
	for _, partialFile := range partialFiles {
		name := strings.TrimPrefix(partialFile, "templates/partials/")
		name = strings.TrimSuffix(name, ".tmpl")

		tmpl, err := template.New(name).Funcs(r.funcMap).ParseFS(templateFS, partialFiles...)
		if err != nil {
			return fmt.Errorf("failed to parse partial template %q: %w", name, err)
		}

		r.templates[name] = tmpl
	}

	return nil
}

// defaultFuncMap returns template helper functions available in all templates.
func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		// Date/time formatting
		"formatDate": func(t time.Time) string {
			return t.Format("Jan 02, 2006")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("Jan 02, 2006 15:04")
		},
		"formatDateTimeFull": func(t time.Time) string {
			return t.Format("Jan 02, 2006 15:04:05 MST")
		},
		"timeAgo": func(t time.Time) string {
			d := time.Since(t)
			switch {
			case d < time.Minute:
				return "just now"
			case d < time.Hour:
				m := int(d.Minutes())
				if m == 1 {
					return "1 minute ago"
				}
				return fmt.Sprintf("%d minutes ago", m)
			case d < 24*time.Hour:
				h := int(d.Hours())
				if h == 1 {
					return "1 hour ago"
				}
				return fmt.Sprintf("%d hours ago", h)
			default:
				days := int(d.Hours() / 24)
				if days == 1 {
					return "1 day ago"
				}
				return fmt.Sprintf("%d days ago", days)
			}
		},

		// String helpers
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": cases.Title(language.English).String,

		// HTML safety — use sparingly and only with trusted content
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},

		// URL safety — use for trusted data URIs (e.g. base64 inline images)
		"safeURL": func(s string) template.URL {
			return template.URL(s)
		},

		// Comparison helpers for templates
		"eq": func(a, b interface{}) bool {
			return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
		},

		// Pointer dereference (useful for *time.Time with timeAgo)
		"deref": func(t *time.Time) time.Time {
			if t == nil {
				return time.Time{}
			}
			return *t
		},

		// Check if a *time.Time is in the past (for expiration checks)
		"isExpired": func(t *time.Time) bool {
			if t == nil {
				return false
			}
			return t.Before(time.Now())
		},

		// Arithmetic (useful for pagination)
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}
}

// HTMLRender implements gin's render.Render interface for a single template execution.
type HTMLRender struct {
	Template *template.Template
	Name     string
	Data     interface{}
}

// Render writes the template to the response writer.
func (h *HTMLRender) Render(w http.ResponseWriter) error {
	h.WriteContentType(w)
	if h.Template == nil {
		return fmt.Errorf("template %q not found", h.Name)
	}
	return h.Template.ExecuteTemplate(w, h.Name, h.Data)
}

// WriteContentType sets the Content-Type header.
func (h *HTMLRender) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{"text/html; charset=utf-8"}
	}
}
