package httpx

import (
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Renderer provides efficient HTML template rendering with lazy loading
// and hot-reload support. Templates are loaded from an fs.FS and cached
// until explicitly reloaded.
//
// Usage:
//
//	//go:embed templates
//	var templatesFS embed.FS
//
//	renderer := httpx.NewRenderer(templatesFS, ".html")
//
//	// Add custom template functions
//	renderer.Funcs(template.FuncMap{
//		"upper": strings.ToUpper,
//	})
//
//	// Render to HTTP response
//	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//		renderer.Html(w, "index", httpx.Vals{
//			"Title": "Home",
//			"User":  "Alice",
//		})
//	})
//
//	// Hot-reload templates during development
//	renderer.Reload()
type Renderer struct {
	dir       fs.FS
	pattern   string
	templates *template.Template
	loaded    atomic.Bool
	mu        sync.Mutex
	funcs     template.FuncMap
}

// NewRenderer creates a new Renderer that loads templates from the given
// filesystem matching the specified pattern (e.g., ".html", ".tmpl").
// Templates are named by their path relative to the filesystem root,
// with the pattern suffix removed.
//
// For example, a file at "pages/index.html" with pattern ".html"
// will be named "pages/index".
func NewRenderer(dir fs.FS, pattern string) *Renderer {
	return &Renderer{
		dir:       dir,
		pattern:   pattern,
		templates: template.New(""),
		funcs:     template.FuncMap{},
	}
}

// Vals is a convenience type for passing data to templates.
type Vals map[string]any

var buffers = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// Funcs registers custom template functions that will be available
// in all templates. This must be called before any templates are rendered.
func (v *Renderer) Funcs(funcs template.FuncMap) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for n, f := range funcs {
		v.funcs[n] = f
	}
}

// Html renders the named template with the given values and writes
// the result to the HTTP response with appropriate headers.
// The Content-Type is set to "text/html; charset=utf-8" and the
// status code is set to 200 OK.
func (v *Renderer) Html(w http.ResponseWriter, template string, vals Vals) error {
	buf := buffers.Get().(*bytes.Buffer)
	err := v.Render(buf, template, vals)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
	return nil
}

// Render executes the named template with the given values and writes
// the output to w. Templates are loaded lazily on first use and cached
// for subsequent renders.
func (v *Renderer) Render(w io.Writer, template string, vals Vals) error {

	if !v.loaded.Load() {
		if err := v.load(); err != nil {
			return err
		}

	}

	return v.templates.ExecuteTemplate(w, template, vals)
}

// Reload marks all templates as stale, forcing them to be reloaded
// on the next render. This is useful for development when templates
// are modified without restarting the application.
func (v *Renderer) Reload() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.templates = template.New("")
	v.loaded.Store(false)
}

func (v *Renderer) load() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.loaded.Load() {
		return nil
	}

	v.templates.Funcs(v.funcs)

	err := fs.WalkDir(v.dir, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if e.IsDir() || filepath.Ext(path) != v.pattern {
			return nil
		}

		buf, err := fs.ReadFile(v.dir, path)
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(path, v.pattern)
		_, err = v.templates.New(name).Parse(string(buf))
		return err
	})

	if err != nil {
		return err
	}

	v.loaded.Store(true)
	return nil
}
