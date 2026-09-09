package notify

import (
	"strings"
	"testing"
)

func TestBuildToastScript_EscapesXMLAndPowerShellSpecialChars(t *testing.T) {
	t.Parallel()
	script := buildToastScript("VPSBackupManager", `Backup failed: <O'Brien's App>`, `Error: "disk full" & unhappy`)

	if strings.Contains(script, "<O'Brien's App>") {
		t.Error("raw title with unescaped XML metacharacters leaked into the script")
	}
	if !strings.Contains(script, "&lt;O&apos;Brien&apos;s App&gt;") {
		t.Errorf("expected XML-escaped title in script, got:\n%s", script)
	}
	if !strings.Contains(script, "&quot;disk full&quot; &amp; unhappy") {
		t.Errorf("expected XML-escaped body in script, got:\n%s", script)
	}
}

func TestPowershellQuote_EscapesSingleQuotes(t *testing.T) {
	t.Parallel()
	got := powershellQuote("it's a test")
	want := "'it''s a test'"
	if got != want {
		t.Errorf("powershellQuote() = %q, want %q", got, want)
	}
}
