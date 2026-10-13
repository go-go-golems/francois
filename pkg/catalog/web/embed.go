// Package web embeds the subscriber portal, admin UI and hosted-origin assets.
package web

import _ "embed"

//go:embed static/index.html
var Index []byte

//go:embed static/catalog.js
var CatalogJS []byte

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

// PortalAsset serves only portal UI files, never subscriber HTML.
func PortalAsset(name string) ([]byte, string, bool) {
	switch name {
	case "catalog.js":
		return CatalogJS, "text/javascript", true
	case "catalog.css":
		return CatalogCSS, "text/css", true
	case "icon.svg":
		return Icon, "image/svg+xml", true
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
	}
	return nil, "", false
}
