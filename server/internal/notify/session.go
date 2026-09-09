package notify

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

// runInActiveConsoleSession launches exe with args in the active console
// session's logged-in user context, not the service's own (Session 0)
// context — a Windows Service cannot show UI or post toast notifications
// in its own session, so this is required for notifications to actually
// reach the user's desktop.
//
// NOTE: this requires the calling process to hold SE_TCB_NAME
// ("Act as part of the operating system"), which the LocalSystem account
// has by default. A service running under a lower-privileged dedicated
// account will not be able to deliver toast notifications this way; that
// tradeoff is documented in README.md.
func runInActiveConsoleSession(exe string, args []string) error {
	sessionID := windows.WTSGetActiveConsoleSessionId()
	const invalidSessionID = 0xFFFFFFFF
	if sessionID == invalidSessionID {
		return fmt.Errorf("no active console session (no user logged in)")
	}

	var userToken windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &userToken); err != nil {
		return fmt.Errorf("query user token for session %d: %w", sessionID, err)
	}
	defer func() { _ = userToken.Close() }()

	var primaryToken windows.Token
	if err := windows.DuplicateTokenEx(
		userToken, windows.MAXIMUM_ALLOWED, nil,
		windows.SecurityImpersonation, windows.TokenPrimary, &primaryToken,
	); err != nil {
		return fmt.Errorf("duplicate user token: %w", err)
	}
	defer func() { _ = primaryToken.Close() }()

	var envBlock *uint16
	if err := windows.CreateEnvironmentBlock(&envBlock, primaryToken, false); err != nil {
		return fmt.Errorf("create environment block: %w", err)
	}
	defer func() { _ = windows.DestroyEnvironmentBlock(envBlock) }()

	cmdLine, err := buildCommandLine(exe, args)
	if err != nil {
		return err
	}
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return fmt.Errorf("encode command line: %w", err)
	}

	startupInfo := &windows.StartupInfo{
		Desktop: windows.StringToUTF16Ptr(`winsta0\default`),
	}
	var procInfo windows.ProcessInformation

	const (
		createUnicodeEnvironment = 0x00000400
		createNoWindow           = 0x08000000
	)
	err = windows.CreateProcessAsUser(
		primaryToken, nil, cmdLinePtr, nil, nil, false,
		createUnicodeEnvironment|createNoWindow, envBlock, nil, startupInfo, &procInfo,
	)
	if err != nil {
		return fmt.Errorf("create process in session %d: %w", sessionID, err)
	}
	defer func() {
		_ = windows.CloseHandle(procInfo.Process)
		_ = windows.CloseHandle(procInfo.Thread)
	}()
	return nil
}

// buildCommandLine quotes exe and args per the Windows command-line
// convention (CommandLineToArgvW), since CreateProcessAsUser takes one
// pre-quoted string rather than an argv slice.
func buildCommandLine(exe string, args []string) (string, error) {
	all := append([]string{exe}, args...)
	quoted := make([]string, len(all))
	for i, a := range all {
		quoted[i] = syscall.EscapeArg(a)
	}
	line := quoted[0]
	for _, a := range quoted[1:] {
		line += " " + a
	}
	if len(line) == 0 {
		return "", fmt.Errorf("empty command line")
	}
	return line, nil
}
