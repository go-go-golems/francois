// Package web embeds the subscriber portal, admin UI and hosted-origin assets.
package web

import (
	"embed"

	local "github.com/go-go-golems/francois/pkg/web"
)

//go:embed static/index.html
var Index []byte

//go:embed static/catalog.js
var CatalogJS []byte

//go:embed static/public.js
var PublicJS []byte

//go:embed static/catalog.css
var CatalogCSS []byte

//go:embed static/launch.js
var LaunchJS []byte

//go:embed static/launch.css
var LaunchCSS []byte

//go:embed static/runtime.js
var RuntimeJS []byte

//go:embed static/icon.svg
var Icon []byte

// Templates are the server-rendered storefront pages (html/template): a
// shared layout plus one file per page.
//
//go:embed templates/*.tmpl
var Templates embed.FS

// LegalTemplate renders /privacy and /terms (html/template).
//
//go:embed legal.tmpl
var LegalTemplate string

// PortalAsset serves only portal UI files, never subscriber HTML.
func PortalAsset(name string) ([]byte, string, bool) {
	switch name {
	case "catalog.js":
		return CatalogJS, "text/javascript", true
	case "catalog.css":
		return CatalogCSS, "text/css", true
	case "icon.svg":
		return Icon, "image/svg+xml", true
	case "public.js":
		return PublicJS, "text/javascript", true
	}
	return nil, "", false
}

// AppAsset serves the trusted bootstrap and runtime on instance origins.
func AppAsset(name string) ([]byte, string, bool) {
	switch name {
	case "launch.js":
		return LaunchJS, "text/javascript", true
	case "launch.css":
		return LaunchCSS, "text/css", true
	case "runtime.js":
		return RuntimeJS, "text/javascript", true
	// Parity with local artifact origins, which applications may reference.
	case "style.css":
		return local.Style, "text/css", true
	case "icon.svg":
		return local.Icon, "image/svg+xml", true
	}
	return nil, "", false
}
