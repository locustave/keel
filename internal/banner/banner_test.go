package banner_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"keel/internal/banner"
)

// captureStdout redirects os.Stdout to a pipe for the duration of fn,
// then returns the captured output as a string.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	orig := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrint_containsNameAndTagline(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	out := captureStdout(func() {
		banner.Print("v0.1.0")
	})

	if !strings.Contains(out, "keel") {
		t.Errorf("expected output to contain %q\ngot:\n%s", "keel", out)
	}
	if !strings.Contains(out, "keeps AI coding sessions on course") {
		t.Errorf("expected output to contain tagline\ngot:\n%s", out)
	}
}

func TestPrint_containsVersion(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	out := captureStdout(func() {
		banner.Print("v1.2.3")
	})

	if !strings.Contains(out, "v1.2.3") {
		t.Errorf("expected output to contain version %q\ngot:\n%s", "v1.2.3", out)
	}
}

func TestPrint_noAnsiWhenNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	out := captureStdout(func() {
		banner.Print("v0.1.0")
	})

	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI escape codes with NO_COLOR=1\ngot:\n%s", out)
	}
}

func TestPrint_pyramidAndStatusPresent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	out := captureStdout(func() {
		banner.Print("v0.1.0")
	})

	for _, want := range []string{
		"██",
		"✓",
		"project structure",
		"gate enforcement",
		"workflow state",
		"execution metrics",
		"initialized",
		"active",
		"tracking",
		"recording",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q\ngot:\n%s", want, out)
		}
	}
}
