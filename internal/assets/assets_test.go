package assets_test

import (
	"io/fs"
	"testing"

	"keel/internal/assets"
)

func TestKeelTemplateContainsREADME(t *testing.T) {
	fsys := assets.KeelTemplate()
	if _, err := fs.Stat(fsys, "assets/keel-template/keel/README.md"); err != nil {
		t.Fatalf("expected assets/keel-template/keel/README.md in embedded FS: %v", err)
	}
}

func TestKeelTemplateContainsRunPhase(t *testing.T) {
	fsys := assets.KeelTemplate()
	if _, err := fs.Stat(fsys, "assets/keel-template/keel/commands/keel-run.md"); err != nil {
		t.Fatalf("expected keel/commands/keel-run.md in embedded FS: %v", err)
	}
}

func TestKeelTemplateContainsRules(t *testing.T) {
	fsys := assets.KeelTemplate()
	entries, err := fs.ReadDir(fsys, "assets/keel-template/keel/rules")
	if err != nil {
		t.Fatalf("could not read keel/rules in template: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("keel/rules is empty in embedded template FS")
	}
}
