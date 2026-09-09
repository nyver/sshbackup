package domain

// Source is one backup source within a job: an absolute remote path plus
// optional include/exclude glob patterns applied when the archive is built.
type Source struct {
	ID         string
	JobID      string
	RemotePath string
	Position   int
	Include    []string
	Exclude    []string
}

// Validate checks that the remote path is absolute, per the backup-jobs
// specification.
func (s *Source) Validate() error {
	return ValidateAbsoluteRemotePath(s.RemotePath)
}
