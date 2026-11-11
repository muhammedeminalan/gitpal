//go:build gui

// Package frontend holds the built web interface of gitpal-gui. Run
// `npm run build` in this directory before building with the gui tag.
package frontend

import "embed"

// Dist is the built interface (frontend/dist).
//
//go:embed all:dist
var Dist embed.FS
