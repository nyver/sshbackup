package domain

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// cronParser matches the standard five-field expression required by the
// scheduling specification (minute hour day-of-month month day-of-week).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ValidateName rejects an empty or whitespace-only name for the given
// entity kind (used in error messages, e.g. "job", "server").
func ValidateName(kind, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s name must not be empty", kind)
	}
	return nil
}

// ValidateAbsoluteRemotePath rejects a relative remote path.
func ValidateAbsoluteRemotePath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("remote path %q must be absolute", path)
	}
	return nil
}

// ValidatePort rejects a TCP port outside the valid 1-65535 range.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d must be between 1 and 65535", port)
	}
	return nil
}

// ValidateCronExpression rejects a cron expression that cannot be parsed as
// a standard five-field expression.
func ValidateCronExpression(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return fmt.Errorf("cron expression must not be empty")
	}
	if _, err := cronParser.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
}

// ValidateTimeoutSeconds rejects a non-positive timeout.
func ValidateTimeoutSeconds(seconds int) error {
	if seconds <= 0 {
		return fmt.Errorf("timeout must be positive, got %d seconds", seconds)
	}
	return nil
}
