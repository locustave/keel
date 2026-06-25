// Package currentphase ports harness/scripts/current_phase.py to Go.
package currentphase

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

var phaseRe = regexp.MustCompile(`(?m)^- phase: (\d+)$`)

func manifestPhases(root string) ([]int, error) {
	path := filepath.Join(root, "BUILD_MANIFEST.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("BUILD_MANIFEST.yaml not found or unreadable: %w", err)
	}
	matches := phaseRe.FindAllStringSubmatch(string(data), -1)
	var phases []int
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		phases = append(phases, n)
	}
	return phases, nil
}

func passedPhases(root string) map[int]bool {
	passed := make(map[int]bool)
	gatesDir := filepath.Join(root, ".agent", "phase_gates")
	entries, err := os.ReadDir(gatesDir)
	if err != nil {
		return passed
	}
	nameRe := regexp.MustCompile(`^phase_(\d+)\.gate\.json$`)
	for _, e := range entries {
		m := nameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(gatesDir, e.Name()))
		if err != nil {
			continue
		}
		var gate map[string]interface{}
		if err := json.Unmarshal(data, &gate); err != nil {
			continue
		}
		if status, _ := gate["status"].(string); status == "passed" {
			n, _ := strconv.Atoi(m[1])
			passed[n] = true
		}
	}
	return passed
}

func failedGateExists(root string, phase int) bool {
	path := filepath.Join(root, ".agent", "phase_gates", fmt.Sprintf("phase_%d.failed.json", phase))
	_, err := os.Stat(path)
	return err == nil
}

// Run reports current phase state to out. Returns exit code (0, 1, or 2).
func Run(repoPath string, out io.Writer) int {
	root, err := filepath.Abs(repoPath)
	if err != nil {
		fmt.Fprintf(out, "ERROR: cannot resolve repo path: %v\n", err)
		return 1
	}

	phases, err := manifestPhases(root)
	if err != nil || len(phases) == 0 {
		fmt.Fprintln(out, "ERROR: BUILD_MANIFEST.yaml not found or has no phases.")
		return 1
	}

	passed := passedPhases(root)
	sort.Ints(phases)

	var remaining []int
	for _, p := range phases {
		if !passed[p] {
			remaining = append(remaining, p)
		}
	}

	var done []int
	for _, p := range phases {
		if passed[p] {
			done = append(done, p)
		}
	}

	if len(remaining) == 0 {
		last := phases[len(phases)-1]
		fmt.Fprintf(out, "All %d phases passed. Last phase: %d.\n", len(phases), last)
		fmt.Fprintln(out, "STATUS: complete")
		return 0
	}

	nextPhase := remaining[0]

	doneStr := "none"
	if len(done) > 0 {
		doneStr = fmt.Sprintf("%v", done)
	}

	if failedGateExists(root, nextPhase) {
		fmt.Fprintf(out, "BLOCKED: phase %d has a failed gate.\n", nextPhase)
		fmt.Fprintf(out, "Passed phases : %s\n", doneStr)
		fmt.Fprintf(out, "Next phase    : %d (blocked — resolve .agent/phase_gates/phase_%d.failed.json)\n", nextPhase, nextPhase)
		fmt.Fprintln(out, "STATUS: blocked")
		return 2
	}

	fmt.Fprintf(out, "Passed phases : %s\n", doneStr)
	fmt.Fprintf(out, "Next phase    : %d\n", nextPhase)
	fmt.Fprintf(out, "Remaining     : %v\n", remaining)
	fmt.Fprintln(out, "STATUS: ready")
	return 0
}
