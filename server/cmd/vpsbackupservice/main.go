// Command vpsbackupservice runs the SSH Backup Manager Windows Service
// host: backup engine, scheduler, and Named Pipes IPC. It also supports a
// --console foreground mode, sharing the same composition root, for
// development and diagnostics.
package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc"
)

// ServiceName is the Windows Service name registered by scripts/.
const ServiceName = "SSH Backup Manager Service"

func main() {
	console := flag.Bool("console", false, "run in the foreground instead of as a Windows Service")
	flag.Parse()

	isService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Fprintln(os.Stderr, "detect Windows Service mode:", err)
		os.Exit(1)
	}

	if *console || !isService {
		if err := runConsole(); err != nil {
			fmt.Fprintln(os.Stderr, "console mode:", err)
			os.Exit(1)
		}
		return
	}

	if err := runService(); err != nil {
		fmt.Fprintln(os.Stderr, "service mode:", err)
		os.Exit(1)
	}
}
