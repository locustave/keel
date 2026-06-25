package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureAndDiff(t *testing.T) {
	repo := t.TempDir()

	// Create some files for the "pre" state.
	writeFile(t, repo, "README.md", "# Hello")
	writeFile(t, repo, "internal/model.go", "package model")
	writeFile(t, repo, "internal/store.go", "package store\nv1")

	pre, err := Capture(repo, 1, "pre")
	if err != nil {
		t.Fatalf("Capture pre: %v", err)
	}

	if len(pre.Files) != 3 {
		t.Errorf("pre has %d files, want 3", len(pre.Files))
	}

	// Simulate phase work: modify one file, create two new files, delete nothing.
	writeFile(t, repo, "internal/store.go", "package store\nv2") // modified
	writeFile(t, repo, "cmd/main.go", "package main")            // created
	writeFile(t, repo, "internal/handler.go", "package handler")  // created

	post, err := Capture(repo, 1, "post")
	if err != nil {
		t.Fatalf("Capture post: %v", err)
	}

	if len(post.Files) != 5 {
		t.Errorf("post has %d files, want 5", len(post.Files))
	}

	diff := Diff(pre, post)

	if len(diff.Created) != 2 {
		t.Errorf("Created = %v, want 2 files", diff.Created)
	}
	if len(diff.Modified) != 1 {
		t.Errorf("Modified = %v, want 1 file", diff.Modified)
	}
	if diff.Modified[0] != "internal/store.go" {
		t.Errorf("Modified[0] = %q, want internal/store.go", diff.Modified[0])
	}
	if len(diff.Deleted) != 0 {
		t.Errorf("Deleted = %v, want 0", diff.Deleted)
	}

	// Verify created files are sorted.
	if diff.Created[0] != "cmd/main.go" || diff.Created[1] != "internal/handler.go" {
		t.Errorf("Created = %v, want [cmd/main.go internal/handler.go]", diff.Created)
	}
}

func TestCaptureSkipsGit(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "README.md", "hello")
	os.MkdirAll(filepath.Join(repo, ".git", "objects"), 0o755)
	writeFile(t, repo, ".git/HEAD", "ref: refs/heads/main")
	writeFile(t, repo, ".git/objects/abc", "blob")

	m, err := Capture(repo, 0, "pre")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	if len(m.Files) != 1 {
		t.Errorf("got %d files, want 1 (only README.md)", len(m.Files))
	}
	if _, ok := m.Files["README.md"]; !ok {
		t.Error("README.md missing from manifest")
	}
}

func TestCaptureSkipsOwnManifests(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "README.md", "hello")
	os.MkdirAll(filepath.Join(repo, ".agent", "snapshots"), 0o755)
	writeFile(t, repo, ".agent/snapshots/phase_1.pre.manifest.json", "{}")

	m, err := Capture(repo, 1, "post")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	for path := range m.Files {
		if path == ".agent/snapshots/phase_1.pre.manifest.json" {
			t.Error("manifest should not include its own snapshot files")
		}
	}
}

func TestWriteAndRead(t *testing.T) {
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".agent", "snapshots"), 0o755)

	m := &Manifest{
		Phase: 2,
		Label: "pre",
		Files: map[string]string{
			"a.go": "abc123",
			"b.go": "def456",
		},
	}

	if err := Write(repo, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	loaded, err := Read(repo, 2, "pre")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if loaded.Phase != 2 {
		t.Errorf("Phase = %d, want 2", loaded.Phase)
	}
	if len(loaded.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(loaded.Files))
	}
	if loaded.Files["a.go"] != "abc123" {
		t.Errorf("a.go hash = %q, want abc123", loaded.Files["a.go"])
	}
}

func TestDiffWithDeletion(t *testing.T) {
	pre := &Manifest{Files: map[string]string{
		"a.go": "aaa",
		"b.go": "bbb",
		"c.go": "ccc",
	}}
	post := &Manifest{Files: map[string]string{
		"a.go": "aaa", // unchanged
		"b.go": "xxx", // modified
		// c.go deleted
		"d.go": "ddd", // created
	}}

	diff := Diff(pre, post)

	if len(diff.Created) != 1 || diff.Created[0] != "d.go" {
		t.Errorf("Created = %v, want [d.go]", diff.Created)
	}
	if len(diff.Modified) != 1 || diff.Modified[0] != "b.go" {
		t.Errorf("Modified = %v, want [b.go]", diff.Modified)
	}
	if len(diff.Deleted) != 1 || diff.Deleted[0] != "c.go" {
		t.Errorf("Deleted = %v, want [c.go]", diff.Deleted)
	}
}

func writeFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, rel)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
