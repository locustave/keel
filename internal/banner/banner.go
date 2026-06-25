package banner

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// colorEnabled returns true when ANSI color codes should be emitted.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// esc wraps text in an ANSI escape sequence and resets afterward.
// Returns text unchanged when color is disabled.
func esc(code, text string, clr bool) string {
	if !clr {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

// Print writes the keel startup banner to stdout.
// version is passed from main via ldflags: -ldflags "-X main.version=v0.1.0"
func Print(version string) {
	clr := colorEnabled()

	dim := func(s string) string       { return esc("2", s, clr) }
	green := func(s string) string     { return esc("32", s, clr) }
	boldWhite := func(s string) string { return esc("1;37", s, clr) }

	// Color only the ██ glyphs, leaving surrounding spaces uncolored.
	colorBlocks := func(s string) string {
		if !clr {
			return s
		}
		return strings.ReplaceAll(s, "██", esc("32", "██", true))
	}

	const divider = "──────────────────────────────────────────────────────"

	// ── header ───────────────────────────────────────────────────────────
	fmt.Fprintf(os.Stdout, "  %s  %s\n", boldWhite("keel"), dim(version))
	fmt.Fprintf(os.Stdout, "  %s\n", dim(divider))

	// ── pyramid rows ─────────────────────────────────────────────────────
	//
	// Each row is printed as:
	//   <2-space indent><pyramid><pad>✓  <feature padded to 20>  <status>
	//
	// The pyramid narrows by one block per row; pad compensates so ✓ lands
	// at display column 21 for every row.
	//
	//   pyramid display widths (assuming ██ = 2 cols):
	//     row0: 3+8+3 = 14   → pad 6
	//     row1: 5+6+2 = 13   → pad 7
	//     row2: 7+4+1 = 12   → pad 8
	//     row3: 9+2   = 11   → pad 9
	type bannerRow struct {
		pyramid string // content after the 2-space indent
		pad     string // spaces bridging pyramid to ✓
		feature string
		status  string
	}

	rows := []bannerRow{
		{" ██ ██ ██ ██", "      ", "project structure", "initialized"},
		{"   ██ ██ ██", "       ", "gate enforcement", "active"},
		{"     ██ ██", "        ", "workflow state", "tracking"},
		{"       ██", "         ", "execution metrics", "recording"},
	}

	for _, r := range rows {
		// Pad feature to 20 chars before applying color so %-20s counts
		// actual runes, not escape-sequence bytes.
		paddedFeature := fmt.Sprintf("%-20s", r.feature)
		fmt.Fprintf(os.Stdout, "  %s%s%s  %s %s\n",
			colorBlocks(r.pyramid),
			r.pad,
			green("✓"),
			dim(paddedFeature),
			green(r.status),
		)
	}

	// ── footer ───────────────────────────────────────────────────────────
	fmt.Fprintf(os.Stdout, "  %s\n", dim(divider))
	fmt.Fprintf(os.Stdout, "  %s\n", dim("keeps AI coding sessions on course"))
}
