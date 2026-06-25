// Package generator contains tests that verify internal consistency of the
// keel template assets embedded in the binary.
//
// Run: go test ./internal/generator/...
package generator_test

import (
	"io/fs"
	"strings"
	"testing"

	"keel/internal/assets"
)

// requiredTemplateFiles are paths that must exist inside the embedded template.
var requiredTemplateFiles = []string{
	"assets/keel-template/agent-rules.md",
	"assets/keel-template/keel/README.md",
	"assets/keel-template/keel/commands/keel-run.md",
	"assets/keel-template/keel/commands/new-feature.md",
	"assets/keel-template/keel/commands/approve-feature.md",
	"assets/keel-template/keel/commands/rollback-phase.md",
	"assets/keel-template/keel/rules/tdd-builder.md",
	"assets/keel-template/keel/rules/stale-plan.md",
	"assets/keel-template/keel/rules/gates.md",
	"assets/keel-template/docs/phases/phase_0.md",
}

// bannedStrings must not appear in any template file (retirement notices exempt).
var bannedStrings = []string{
	"HARNESS_EXECUTION_PLAN.yaml",
	"harness/README.md",
}

var retirementExceptions = []string{
	"retired",
	"Do not",
	"is retired",
}

func TestRequiredTemplateFilesExist(t *testing.T) {
	fsys := assets.KeelTemplate()
	for _, path := range requiredTemplateFiles {
		if _, err := fs.Stat(fsys, path); err != nil {
			t.Errorf("missing required template file: %s", path)
		}
	}
}

func TestNoBannedStringsInTemplate(t *testing.T) {
	fsys := assets.KeelTemplate()

	err := fs.WalkDir(fsys, "assets/keel-template", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		for _, banned := range bannedStrings {
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.Contains(line, banned) {
					continue
				}
				exempt := false
				for _, ex := range retirementExceptions {
					if strings.Contains(strings.ToLower(line), strings.ToLower(ex)) {
						exempt = true
						break
					}
				}
				if !exempt {
					t.Errorf("banned string %q in %s:\n  %s", banned, path, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("error walking template: %v", err)
	}
}
