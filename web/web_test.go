package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strings"
	"testing"
)

// index.html has to load the app and stylesheets that exist.
func TestIndexLoadsApp(t *testing.T) {
	index, err := fs.ReadFile(Assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`(?:src|href)="([^":]+)"`).FindAllSubmatch(index, -1)
	app := false
	for _, m := range refs {
		name := string(m[1])
		app = app || name == "app.js"
		if _, err := fs.Stat(Assets, name); err != nil {
			t.Errorf("index.html loads %s: %v", name, err)
		}
	}
	if !app {
		t.Error("index.html does not load app.js")
	}
}

var (
	jsImport  = regexp.MustCompile(`(?:from\s+|import\s*\(\s*|import\s+)'([^']+)'`)
	cssImport = regexp.MustCompile(`@import url\(([^)]+)\)`)
)

// imports reads the import map of index.html: bare specifier prefixes and
// the paths they stand for.
func imports(t *testing.T) map[string]string {
	index, err := fs.ReadFile(Assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Imports map[string]string }
	for _, found := range importMap.FindAllSubmatch(index, -1) {
		if err := json.Unmarshal(found[1], &m); err != nil {
			t.Fatalf("import map: %v", err)
		}
	}
	return m.Imports
}

// Every module a page imports, directly or through the import map, has to be
// in the embedded tree. A missing node_modules means the browser
// dependencies were not installed.
func TestImportsResolve(t *testing.T) {
	if _, err := fs.Stat(Assets, "node_modules/@lsongdev/scripts/package.json"); err != nil {
		t.Fatalf("browser dependencies are missing, run make deps: %v", err)
	}
	mapped := imports(t)
	resolve := func(from, spec string) (string, bool) {
		if strings.HasPrefix(spec, ".") {
			return path.Join(path.Dir(from), spec), true
		}
		for prefix, target := range mapped {
			if strings.HasPrefix(spec, prefix) {
				return path.Clean(strings.TrimPrefix(target, "./") + strings.TrimPrefix(spec, prefix)), true
			}
		}
		return "", false
	}
	err := fs.WalkDir(Assets, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		var pattern *regexp.Regexp
		switch path.Ext(p) {
		case ".js":
			pattern = jsImport
		case ".css":
			pattern = cssImport
		default:
			return nil
		}
		data, err := fs.ReadFile(Assets, p)
		if err != nil {
			return err
		}
		for _, m := range pattern.FindAllSubmatch(data, -1) {
			target, ok := resolve(p, string(m[1]))
			if !ok {
				t.Errorf("%s imports %s, which neither is relative nor in the import map", p, m[1])
				continue
			}
			if _, err := fs.Stat(Assets, target); err != nil {
				t.Errorf("%s imports %s: %v", p, m[1], err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The policy allows the import map by its hash, so browsers apply it.
func TestPolicyAllowsImportMap(t *testing.T) {
	w := httptest.NewRecorder()
	Handler(echo{}, "").ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'self' 'sha256-") {
		t.Errorf("policy = %q", got)
	}
}

type echo struct{}

func (echo) ServeHTTP(w http.ResponseWriter, r *http.Request) { w.Write([]byte("api:" + r.URL.Path)) }
