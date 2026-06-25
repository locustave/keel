// Package merge merges a feature BUILD_MANIFEST.yaml into the project BUILD_MANIFEST.yaml.
// It is a port of scripts/merge_feature.py with both known bugs fixed:
//   1. find_phases regex is indentation-agnostic (matches "- phase:" with any leading whitespace).
//   2. renumber_phases applies replacements in descending order to avoid cascade renaming.
package merge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var rePhase = regexp.MustCompile(`(?m)^\s*- phase: (\d+)\s*$`)

// findPhases returns all phase numbers found in text.
func findPhases(text string) []int {
	var nums []int
	for _, m := range rePhase.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			nums = append(nums, n)
		}
	}
	return nums
}

// lastPhase returns the highest phase number in text, or -1 if none.
func lastPhase(text string) int {
	phases := findPhases(text)
	if len(phases) == 0 {
		return -1
	}
	max := phases[0]
	for _, n := range phases[1:] {
		if n > max {
			max = n
		}
	}
	return max
}

var rePhaseLine = regexp.MustCompile(`(?m)^(\s*- phase: )(\d+)(\s*)$`)

// splitPhasesBlock returns (preamble, phases_block) where phases_block starts at the
// first "  - phase:" line. Matches two-space-indented feature manifest format.
func splitPhasesBlock(text string) (string, string) {
	re := regexp.MustCompile(`(?m)^  - phase: \d+`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return text, ""
	}
	return text[:loc[0]], text[loc[0]:]
}

// dedentPhases removes `spaces` leading spaces from every line.
func dedentPhases(text string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	var sb strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			sb.WriteString(line[spaces:])
		} else {
			sb.WriteString(line)
		}
	}
	return sb.String()
}

// renumberPhases replaces phase numbers in featureText starting from start.
// Builds mapping ascending, applies replacements descending to avoid cascade.
// Returns (newText, mapping of old→new).
func renumberPhases(featureText string, start int) (string, [][2]int) {
	featurePhases := findPhases(featureText)
	if len(featurePhases) == 0 {
		return featureText, nil
	}

	sorted := make([]int, len(featurePhases))
	copy(sorted, featurePhases)
	sort.Ints(sorted)

	// Deduplicate
	var uniq []int
	for i, v := range sorted {
		if i == 0 || v != sorted[i-1] {
			uniq = append(uniq, v)
		}
	}

	mapping := make([][2]int, len(uniq))
	for i, old := range uniq {
		mapping[i] = [2]int{old, start + i}
	}

	// Sort descending by old value to avoid cascade
	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i][0] > mapping[j][0]
	})

	result := featureText
	for _, pair := range mapping {
		old, new := pair[0], pair[1]
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^(\s*- phase: )%d(\s*)$`, old))
		result = re.ReplaceAllString(result, fmt.Sprintf("${1}%d${2}", new))
	}

	// Normalise indentation to match project manifest (0-space phase entries)
	result = dedentPhases(result, 2)

	// Re-sort mapping ascending for output
	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i][0] < mapping[j][0]
	})

	return result, mapping
}

// Options controls merge behaviour.
type Options struct {
	Slug    string
	Confirm bool
	RepoDir string    // defaults to "." (resolved)
	Out     io.Writer // for dry-run output (nil → os.Stdout)
	Err     io.Writer // for error output (nil → os.Stderr)
}

// Run merges the feature manifest into the project manifest. Returns 0/1/2.
func Run(opts Options) int {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Err == nil {
		opts.Err = os.Stderr
	}
	if opts.Slug == "" {
		fmt.Fprintln(opts.Err, "Usage: harness merge-feature <slug> [--confirm]")
		return 2
	}

	root := opts.RepoDir
	if root == "" {
		var err error
		root, err = filepath.Abs(".")
		if err != nil {
			fmt.Fprintf(opts.Err, "ERROR: %v\n", err)
			return 1
		}
	}

	projectPath := filepath.Join(root, "BUILD_MANIFEST.yaml")
	featurePath := filepath.Join(root, "docs", "features", opts.Slug, "BUILD_MANIFEST.yaml")

	projectData, err := os.ReadFile(projectPath)
	if err != nil {
		fmt.Fprintf(opts.Err, "ERROR: %s not found.\n", projectPath)
		return 1
	}
	featureData, err := os.ReadFile(featurePath)
	if err != nil {
		fmt.Fprintf(opts.Err, "ERROR: %s not found.\n", featurePath)
		return 1
	}

	projectText := string(projectData)
	featureText := string(featureData)

	featurePhases := findPhases(featureText)
	if len(featurePhases) == 0 {
		fmt.Fprintf(opts.Err, "ERROR: No phases found in %s.\n", featurePath)
		return 1
	}

	start := lastPhase(projectText) + 1
	_, phasesBlock := splitPhasesBlock(featureText)
	renumbered, mapping := renumberPhases(phasesBlock, start)

	merged := strings.TrimRight(projectText, "\n") + "\n" + renumbered
	if !strings.HasSuffix(merged, "\n") {
		merged += "\n"
	}

	fmt.Fprintf(opts.Out, "Feature   : %s\n", opts.Slug)
	fmt.Fprintf(opts.Out, "Source    : docs/features/%s/BUILD_MANIFEST.yaml\n", opts.Slug)
	var mapParts []string
	for _, pair := range mapping {
		mapParts = append(mapParts, fmt.Sprintf("%d→%d", pair[0], pair[1]))
	}
	fmt.Fprintf(opts.Out, "Mapping   : %s\n", strings.Join(mapParts, ", "))
	var newPhases []string
	for _, pair := range mapping {
		newPhases = append(newPhases, strconv.Itoa(pair[1]))
	}
	fmt.Fprintf(opts.Out, "Inserting : phases %s into BUILD_MANIFEST.yaml\n", strings.Join(newPhases, ", "))

	if !opts.Confirm {
		fmt.Fprintln(opts.Out, "\nDry run — pass --confirm to write.")
		fmt.Fprintln(opts.Out, "\nPhases that would be added:")
		for _, pair := range mapping {
			fmt.Fprintf(opts.Out, "  phase %d\n", pair[1])
		}
		return 0
	}

	if err := os.WriteFile(projectPath, []byte(merged), 0o644); err != nil {
		fmt.Fprintf(opts.Err, "ERROR: %v\n", err)
		return 1
	}
	fmt.Fprintf(opts.Out, "\nMerged. First new phase: %d.\n", mapping[0][1])
	return 0
}
