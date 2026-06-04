package admin

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

func LoadTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n {
				return s
			}
			return string(runes[:n]) + "…"
		},
	}
	return template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
}
