// Package assets exposes the embedded static keel assets as io/fs.FS values.
// The actual //go:embed declarations live in embed.go at the module root,
// because Go forbids '..' in embed paths and all asset directories sit at
// the repo root.
package assets

import (
	"io/fs"

	staticassets "keel"
)

// KeelTemplate returns the embedded keel template tree.
// Callers should sub-FS into "assets/keel-template" before walking.
func KeelTemplate() fs.FS {
	return staticassets.KeelTemplateFS
}
