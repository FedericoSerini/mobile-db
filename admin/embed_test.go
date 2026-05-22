package admin_test

import (
	"html/template"
	"strings"
	"testing"

	"github.com/federicoserini/mobile-db/admin"
)

func TestTemplatesLoad(t *testing.T) {
	tmpl, err := admin.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if tmpl.Lookup("base") == nil {
		t.Fatal("template 'base' must be defined")
	}
}

func TestBaseTemplateRenders(t *testing.T) {
	tmpl, _ := admin.LoadTemplates()
	var buf strings.Builder
	err := tmpl.ExecuteTemplate(&buf, "base", map[string]any{
		"Title":   "Test Page",
		"Content": template.HTML("hello"),
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	if !strings.Contains(buf.String(), "Test Page") {
		t.Fatal("rendered output must contain page title")
	}
}
