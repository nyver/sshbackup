package domain

import "errors"

// DefaultScriptTimeoutSeconds is applied when a script is added without an
// explicit timeout.
const DefaultScriptTimeoutSeconds = 300

// Script is one command executed on the remote host before or after the
// backup archive is created. Command text is stored and executed exactly
// as entered: the system never rewrites, escapes, or reorders it.
type Script struct {
	ID              string
	JobID           string
	Type            ScriptType
	Command         string
	Position        int
	TimeoutSeconds  int
	RunCondition    RunCondition
	CriticalCleanup bool
	RetryOnFailure  bool
}

// Validate checks the invariants required by the backup-jobs specification.
func (s *Script) Validate() error {
	var errs []error
	if s.Type != ScriptPreBackup && s.Type != ScriptPostBackup {
		errs = append(errs, errors.New("script type must be PRE_BACKUP or POST_BACKUP"))
	}
	if s.Command == "" {
		errs = append(errs, errors.New("script command must not be empty"))
	}
	if err := ValidateTimeoutSeconds(s.TimeoutSeconds); err != nil {
		errs = append(errs, err)
	}
	switch s.RunCondition {
	case RunConditionOnSuccess, RunConditionOnFailure, RunConditionAlways:
	default:
		errs = append(errs, errors.New("script run condition must be ON_SUCCESS, ON_FAILURE, or ALWAYS"))
	}
	return errors.Join(errs...)
}

// ShouldRun reports whether the script should execute given whether the run
// has failed so far.
func (s *Script) ShouldRun(runFailedSoFar bool) bool {
	switch s.RunCondition {
	case RunConditionAlways:
		return true
	case RunConditionOnFailure:
		return runFailedSoFar
	case RunConditionOnSuccess:
		return !runFailedSoFar
	default:
		return false
	}
}
