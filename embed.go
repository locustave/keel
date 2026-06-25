// Package staticassets embeds all static keel assets into the binary.
// This file must live at the module root so that //go:embed paths resolve
// directly to the repo-root asset directories without using '..'.
package staticassets

import "embed"

// KeelTemplateFS contains the full keel template tree copied into
// target projects by keel init.
//
//go:embed all:assets/keel-template
var KeelTemplateFS embed.FS
