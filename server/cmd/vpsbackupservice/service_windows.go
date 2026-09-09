package main

import (
	"context"

	"golang.org/x/sys/windows/svc"
)

// windowsServiceHandler implements svc.Handler, sharing the exact same
// startup/shutdown sequence as console mode via the same composition
// root (startup/App.Shutdown).
type windowsServiceHandler struct{}

func runService() error {
	return svc.Run(ServiceName, windowsServiceHandler{})
}

func (windowsServiceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	s <- svc.Status{State: svc.StartPending}

	app, err := startup(context.Background(), "service")
	if err != nil {
		// A failure to open or migrate the database must prevent the
		// service from reporting itself as started, per the
		// service-lifecycle specification.
		s <- svc.Status{State: svc.Stopped, Win32ExitCode: 1}
		return false, 1
	}

	const accepts = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.Running, Accepts: accepts}

loop:
	for req := range r {
		switch req.Cmd {
		case svc.Interrogate:
			s <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			s <- svc.Status{State: svc.StopPending}
			app.Shutdown()
			break loop
		}
	}

	s <- svc.Status{State: svc.Stopped}
	return false, 0
}
