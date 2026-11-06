package web

import (
	"io/fs"
	"path"
	"regexp"
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
	jsImport  = regexp.MustCompile(`(?:from\s+|import\s*\(\s*|import\s+)'(\.[^']+)'`)
	cssImport = regexp.MustCompile(`@import url\(([^)]+)\)`)
)

// Every module a page imports has to be in the embedded tree. A missing
// node_modules means the browser dependencies were not installed.
func TestImportsResolve(t *testing.T) {
	if _, err := fs.Stat(Assets, "node_modules/@lsongdev/scripts/package.json"); err != nil {
		t.Fatalf("browser dependencies are missing, run make deps: %v", err)
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
			target := path.Join(path.Dir(p), string(m[1]))
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
