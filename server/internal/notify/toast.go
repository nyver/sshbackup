package notify

import (
	"context"
	"fmt"
	"strings"
)

// DefaultAppID is shown as the toast's source application when no custom
// Application User Model ID is configured.
const DefaultAppID = "VPSBackupManager"

// WindowsToastNotifier shows a Windows toast notification by launching a
// short PowerShell script, in the active console session's user context,
// that calls the WinRT Windows.UI.Notifications APIs directly (no
// third-party notification module required).
type WindowsToastNotifier struct {
	AppID string
}

func (w *WindowsToastNotifier) appID() string {
	if w.AppID != "" {
		return w.AppID
	}
	return DefaultAppID
}

// Notify shows n as a toast. The context is accepted for interface
// symmetry with other I/O-bound Notifiers; the underlying Win32 call is
// not itself cancellable.
func (w *WindowsToastNotifier) Notify(_ context.Context, n Notification) error {
	script := buildToastScript(w.appID(), n.Title, n.Body)
	if err := runInActiveConsoleSession("powershell.exe", []string{
		"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script,
	}); err != nil {
		return fmt.Errorf("show toast notification: %w", err)
	}
	return nil
}

// buildToastScript renders a self-contained PowerShell script that posts
// one toast via the WinRT toast APIs. title and body are embedded as
// PowerShell single-quoted string literals (doubled single quotes, the
// PowerShell escaping convention), never interpolated as code.
func buildToastScript(appID, title, body string) string {
	var b strings.Builder
	b.WriteString("[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType=WindowsRuntime] | Out-Null;\n")
	b.WriteString("[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType=WindowsRuntime] | Out-Null;\n")
	fmt.Fprintf(&b, "$template = @\"\n<toast><visual><binding template=\"ToastGeneric\"><text>%s</text><text>%s</text></binding></visual></toast>\n\"@;\n",
		xmlEscape(title), xmlEscape(body))
	b.WriteString("$xml = New-Object Windows.Data.Xml.Dom.XmlDocument;\n")
	b.WriteString("$xml.LoadXml($template);\n")
	b.WriteString("$toast = New-Object Windows.UI.Notifications.ToastNotification $xml;\n")
	fmt.Fprintf(&b, "[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier(%s).Show($toast);\n", powershellQuote(appID))
	return b.String()
}

func powershellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
