// Package snapshot captures and compares file manifests for phase rollback.
//
// A manifest is a map of relative file paths to their SHA-256 checksums,
// captured at phase boundaries. Comparing the pre-phase and post-phase
// manifests tells rollback exactly which files were created vs modified.
//
// At phase start, a full file stash is also written — a copy of every file
// in the working tree. This allows rollback to restore modified files
// without any dependency on git.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest maps relative file paths to their SHA-256 hex digests.
type Manifest struct {
	Phase int               `json:"phase"`
	Label string            `json:"label"` // "pre" or "post"
	Files map[string]string `json:"files"` // path → sha256
}

// DiffResult holds the outcome of comparing two manifests.
type DiffResult struct {
	Created  []string // files in post but not in pre
	Modified []string // files in both but with different checksums
	Deleted  []string // files in pre but not in post
}

// Capture walks repoPath and builds a manifest of all tracked files.
// It skips .git/, binary-looking files over 10MB, and the snapshot
// files themselves to avoid self-referencing.
func Capture(repoPath string, phase int, label string) (*Manifest, error) {
	repo, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}

	files := make(map[string]string)

	err = filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(repo, path)

		if info.IsDir() {
			if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || rel == ".git" {
			return nil
		}

		// Skip snapshot artifacts to avoid self-referencing.
		if isSnapshotArtifact(rel) {
			return nil
		}

		if info.Size() > 10*1024*1024 {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return nil
		}

		files[rel] = hash
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Manifest{
		Phase: phase,
		Label: label,
		Files: files,
	}, nil
}

// isSnapshotArtifact returns true for files inside the snapshots directory
// that are managed by this package.
func isSnapshotArtifact(rel string) bool {
	return strings.Contains(rel, ".manifest.json") ||
		strings.Contains(rel, filepath.Join(".agent", "snapshots", "stash_"))
}

// Diff compares a pre-phase manifest against a post-phase manifest.
func Diff(pre, post *Manifest) *DiffResult {
	result := &DiffResult{}

	for path, postHash := range post.Files {
		preHash, existed := pre.Files[path]
		if !existed {
			result.Created = append(result.Created, path)
		} else if preHash != postHash {
			result.Modified = append(result.Modified, path)
		}
	}

	for path := range pre.Files {
		if _, exists := post.Files[path]; !exists {
			result.Deleted = append(result.Deleted, path)
		}
	}

	sort.Strings(result.Created)
	sort.Strings(result.Modified)
	sort.Strings(result.Deleted)
	return result
}

// ManifestPath returns the canonical path for a phase manifest file.
func ManifestPath(repoPath string, phase int, label string) string {
	return filepath.Join(repoPath, ".agent", "snapshots",
		fmt.Sprintf("phase_%d.%s.manifest.json", phase, label))
}

// Write serializes a manifest to disk.
func Write(repoPath string, m *Manifest) error {
	path := ManifestPath(repoPath, m.Phase, m.Label)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Read loads a manifest from disk.
func Read(repoPath string, phase int, label string) (*Manifest, error) {
	path := ManifestPath(repoPath, phase, label)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ---------------------------------------------------------------------------
// File stash — full copy of every file at phase start
// ---------------------------------------------------------------------------

// StashDir returns the path to the stash directory for a phase.
func StashDir(repoPath string, phase int) string {
	return filepath.Join(repoPath, ".agent", "snapshots", fmt.Sprintf("stash_%d", phase))
}

// Stash copies every file in the manifest to a stash directory.
// Called at phase start so rollback can restore modified files without git.
func Stash(repoPath string, manifest *Manifest) error {
	repo, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	stashDir := StashDir(repo, manifest.Phase)

	for rel := range manifest.Files {
		src := filepath.Join(repo, rel)
		dst := filepath.Join(stashDir, rel)

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("stash mkdir %s: %w", rel, err)
		}

		if err := copyFile(src, dst); err != nil {
			// Skip files that vanished between manifest capture and stash.
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stash copy %s: %w", rel, err)
		}
	}
	return nil
}

// RestoreFile copies a single file from the stash back to the working tree.
func RestoreFile(repoPath string, phase int, relPath string) error {
	repo, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	src := filepath.Join(StashDir(repo, phase), relPath)
	dst := filepath.Join(repo, relPath)

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyFile(src, dst)
}

// CleanStash removes the stash directory for a phase.
func CleanStash(repoPath string, phase int) {
	os.RemoveAll(StashDir(repoPath, phase))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
