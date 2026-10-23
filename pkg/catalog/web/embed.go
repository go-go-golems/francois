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

// Raster icons rendered from icon.svg (rsvg-convert, magick) for browsers
// and home screens that do not use the SVG.
//
//go:embed static/favicon.ico
var Favicon []byte

//go:embed static/apple-touch-icon.png
var AppleTouchIcon []byte

//go:embed static/icon-512.png
var Icon512 []byte

// Templates are the server-rendered storefront pages (html/template): a
// shared layout plus one file per page.
//
//go:embed templates/*.tmpl
var Templates embed.FS

// Separate allowlists keep subscriber HTML out of the public asset routes.
var portalAssets = map[string]*Asset{
	"catalog.js":           newAsset("catalog.js", "text/javascript", CatalogJS),
	"catalog.css":          newAsset("catalog.css", "text/css", CatalogCSS),
	"icon.svg":             newAsset("icon.svg", "image/svg+xml", Icon),
	"public.js":            newAsset("public.js", "text/javascript", PublicJS),
	"favicon.ico":          newAsset("favicon.ico", "image/x-icon", Favicon),
	"apple-touch-icon.png": newAsset("apple-touch-icon.png", "image/png", AppleTouchIcon),
	"icon-512.png":         newAsset("icon-512.png", "image/png", Icon512),
}

var appAssets = map[string]*Asset{
	"launch.js":  newAsset("launch.js", "text/javascript", LaunchJS),
	"launch.css": newAsset("launch.css", "text/css", LaunchCSS),
	"runtime.js": newAsset("runtime.js", "text/javascript", RuntimeJS),
	"style.css":  newAsset("style.css", "text/css", local.Style),
	"icon.svg":   newAsset("icon.svg", "image/svg+xml", local.Icon),
}

func PortalAsset(name string) (*Asset, bool) { a, ok := portalAssets[name]; return a, ok }
func AppAsset(name string) (*Asset, bool)    { a, ok := appAssets[name]; return a, ok }

func PortalURL(name string) string { return portalAssets[name].URL() }
func AppURL(name string) string    { return appAssets[name].URL() }

// PortalIndex references the same content-versioned assets as the storefront.
var PortalIndex = versionRefs(Index, portalAssets)
