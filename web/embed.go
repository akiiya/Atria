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

// StaticDist returns the embedded dist filesystem (Vue SPA build output).
// Returns error if dist directory doesn't exist (frontend not built).
func StaticDist() (fs.FS, error) {
	sub, err := fs.Sub(staticFS, "static/dist")
	if err != nil {
		return nil, err
	}
	// Check if index.html exists
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, err
	}
	return sub, nil
}
