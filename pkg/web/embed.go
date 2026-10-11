// Package web contains the embedded Francois browser assets.
package web

import _ "embed"

//go:embed static/index.html
var Index []byte

//go:embed static/app.js
var App []byte

//go:embed static/runtime.js
var Runtime []byte

//go:embed static/style.css
var Style []byte

//go:embed static/icon.svg
var Icon []byte
