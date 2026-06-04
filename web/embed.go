package web

import (
	"embed"
	"io/fs"
)

//go:embed all:templates
var templatesFS embed.FS

//go:embed all:static
var staticFS embed.FS

// TemplatesFS returns the embedded templates filesystem.
func TemplatesFS() (embed.FS, error) {
	return templatesFS, nil
}

// Static returns the embedded static filesystem.
func Static() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}
