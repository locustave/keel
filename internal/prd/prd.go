// Package prd discovers PRD files in a repository and prompts the user
// to select one or create a new PRD during keel init.
package prd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// prdExtensions lists the file extensions we search for.
var prdExtensions = []string{".md", ".pdf", ".docx", ".html"}

// FindPRDs searches repoPath (recursively, up to 3 levels deep) for files
// whose base name starts with "prd" (case-insensitive) and has a recognised
// extension. It returns paths relative to repoPath.
func FindPRDs(repoPath string) []string {
	var results []string
	root, _ := filepath.Abs(repoPath)

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip hidden dirs and node_modules, and limit depth.
			name := d.Name()
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "keel" {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if rel != "." && strings.Count(rel, string(filepath.Separator)) >= 3 {
				return filepath.SkipDir
			}
			return nil
		}

		base := strings.ToLower(d.Name())
		ext := strings.ToLower(filepath.Ext(base))
		nameNoExt := strings.TrimSuffix(base, ext)

		// Match filenames like prd.md, prd-v2.md, product_requirements.docx, etc.
		isPRD := nameNoExt == "prd" || strings.HasPrefix(nameNoExt, "prd_") ||
			strings.HasPrefix(nameNoExt, "prd-") || strings.HasPrefix(nameNoExt, "prd.")

		if !isPRD {
			return nil
		}
		for _, want := range prdExtensions {
			if ext == want {
				rel, _ := filepath.Rel(root, path)
				results = append(results, rel)
				return nil
			}
		}
		return nil
	})

	return results
}

// PromptSelect shows the user a numbered list of discovered PRD files and
// returns the selected path (relative to repoPath). Returns empty string if
// the user declines all options.
func PromptSelect(prds []string, in io.Reader, out io.Writer) string {
	if !isTerminal(in) {
		// Non-interactive: pick the first one.
		fmt.Fprintf(out, "  (non-interactive — using %s)\n", prds[0])
		return prds[0]
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Found PRD files:")
	fmt.Fprintln(out)
	for i, p := range prds {
		fmt.Fprintf(out, "    [%d] %s\n", i+1, p)
	}
	fmt.Fprintln(out)

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "  Select PRD [1]: ")
		scanner.Scan()
		text := strings.TrimSpace(scanner.Text())

		if text == "" {
			return prds[0]
		}

		var idx int
		if _, err := fmt.Sscanf(text, "%d", &idx); err == nil && idx >= 1 && idx <= len(prds) {
			return prds[idx-1]
		}
		fmt.Fprintln(out, "  Invalid selection, try again.")
	}
}

// PromptCreate asks the user for project details and writes a docs/PRD.md
// file. Returns the relative path "docs/PRD.md" on success.
func PromptCreate(repoPath string, in io.Reader, out io.Writer) (string, error) {
	if !isTerminal(in) {
		return "", fmt.Errorf("no PRD found and running non-interactively — pass --prd or create docs/PRD.md")
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  No PRD found. Let's create one.")
	fmt.Fprintln(out)

	scanner := bufio.NewScanner(in)

	name := promptField(scanner, out, "  Project name")
	description := promptField(scanner, out, "  One-line description")
	goals := promptField(scanner, out, "  Key goals (comma-separated)")
	audience := promptField(scanner, out, "  Target users / audience")
	features := promptField(scanner, out, "  Core features (comma-separated)")

	content := buildPRD(name, description, goals, audience, features)

	prdDir := filepath.Join(repoPath, "docs")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create docs/: %w", err)
	}

	prdPath := filepath.Join(prdDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("cannot write PRD: %w", err)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Created docs/PRD.md")

	return "docs/PRD.md", nil
}

// CopyToDocsPRD copies the selected PRD into docs/PRD.md (if it isn't already
// there). Returns the destination relative path.
func CopyToDocsPRD(repoPath, relPath string) error {
	if relPath == "docs/PRD.md" || relPath == filepath.Join("docs", "PRD.md") {
		return nil // already in the right place
	}

	src := filepath.Join(repoPath, relPath)
	dstDir := filepath.Join(repoPath, "docs")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dstDir, "PRD.md")

	// Don't overwrite an existing docs/PRD.md.
	if _, err := os.Stat(dst); err == nil {
		return nil
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func promptField(scanner *bufio.Scanner, out io.Writer, label string) string {
	fmt.Fprintf(out, "%s: ", label)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func buildPRD(name, description, goals, audience, features string) string {
	var b strings.Builder

	if name == "" {
		name = "Untitled Project"
	}

	b.WriteString(fmt.Sprintf("# %s — Product Requirements Document\n\n", name))

	b.WriteString("## Overview\n\n")
	if description != "" {
		b.WriteString(description + "\n\n")
	} else {
		b.WriteString("_TODO: Describe the product._\n\n")
	}

	b.WriteString("## Goals\n\n")
	if goals != "" {
		for _, g := range strings.Split(goals, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				b.WriteString(fmt.Sprintf("- %s\n", g))
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- _TODO: Add project goals._\n\n")
	}

	b.WriteString("## Target Audience\n\n")
	if audience != "" {
		b.WriteString(audience + "\n\n")
	} else {
		b.WriteString("_TODO: Describe the target audience._\n\n")
	}

	b.WriteString("## Core Features\n\n")
	if features != "" {
		for _, f := range strings.Split(features, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				b.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- _TODO: List core features._\n\n")
	}

	b.WriteString("## Non-Goals\n\n")
	b.WriteString("- _TODO: What is explicitly out of scope._\n\n")

	b.WriteString("## Success Metrics\n\n")
	b.WriteString("- _TODO: How will success be measured._\n")

	return b.String()
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
