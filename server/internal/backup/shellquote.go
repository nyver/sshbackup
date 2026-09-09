package backup

import "strings"

// shellQuote wraps s in single quotes for safe inclusion in a POSIX shell
// command, per design.md: every user-supplied path and pattern goes
// through single-quote escaping, while script command text itself is
// passed through verbatim and unescaped (a deliberate product decision).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
