package retention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

type memRunStore struct {
	mu   sync.Mutex
	runs map[string][]*domain.Run // jobID -> runs, most-recent-first (as ListArchived returns)
}

func newMemRunStore(runs ...*domain.Run) *memRunStore {
	s := &memRunStore{runs: make(map[string][]*domain.Run)}
	for _, r := range runs {
		s.runs[r.JobID] = append(s.runs[r.JobID], r)
	}
	return s
}

func (s *memRunStore) ListArchived(_ context.Context, jobID string) ([]*domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*domain.Run
	for _, r := range s.runs[jobID] {
		if r.ArchiveName != "" {
			out = append(out, r)
		}
	}
	// Match store.RunRepository.ListArchived's ORDER BY started_at DESC.
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

func (s *memRunStore) Update(_ context.Context, run *domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.runs[run.JobID] {
		if r.ID == run.ID {
			*r = *run
			return nil
		}
	}
	return nil
}

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

func newRunWithArchive(t *testing.T, jobID, destDir, name string, startedAt time.Time) *domain.Run {
	t.Helper()
	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("domain.NewID() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, name), []byte("archive"), 0o600); err != nil {
		t.Fatalf("write fixture archive: %v", err)
	}
	return &domain.Run{ID: id, JobID: jobID, ArchiveName: name, StartedAt: startedAt, Status: domain.RunSuccess}
}

func testJob(destDir string, keepLast, maxAgeDays *int) *domain.Job {
	return &domain.Job{
		ID: "job-1", Name: "beresta", LocalDestination: destDir,
		RetentionPolicy: domain.RetentionPolicy{KeepLast: keepLast, MaxAgeDays: maxAgeDays},
	}
}

func intPtr(v int) *int { return &v }

func TestApplier_Apply_NoPolicyConfigured(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	run := newRunWithArchive(t, "job-1", dir, "a.tar.gz", now.AddDate(0, 0, -100))

	runs := newMemRunStore(run)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job := testJob(dir, nil, nil)

	deleted, warnings, err := a.Apply(context.Background(), job)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if deleted != 0 || len(warnings) != 0 {
		t.Fatalf("deleted=%d warnings=%v, want no deletions when no policy is configured", deleted, warnings)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.tar.gz")); err != nil {
		t.Error("archive file should still exist")
	}
}

func TestApplier_Apply_KeepLast(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	// 11 archives, most-recent-first; keep_last=10 should delete only the
	// oldest one.
	var runsList []*domain.Run
	for i := 0; i < 11; i++ {
		name := fmtArchiveName(i)
		r := newRunWithArchive(t, "job-1", dir, name, now.AddDate(0, 0, -i))
		runsList = append(runsList, r)
	}
	runs := newMemRunStore(runsList...)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job := testJob(dir, intPtr(10), nil)

	deleted, warnings, err := a.Apply(context.Background(), job)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	// The oldest (index 10, "archive-10.tar.gz") must be gone.
	if _, err := os.Stat(filepath.Join(dir, fmtArchiveName(10))); !os.IsNotExist(err) {
		t.Error("expected the oldest archive to be deleted")
	}
	// The newest nine plus the 10th (index 9) must remain: 10 total.
	remaining := 0
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, fmtArchiveName(i))); err == nil {
			remaining++
		}
	}
	if remaining != 10 {
		t.Errorf("remaining archives = %d, want 10", remaining)
	}
}

func fmtArchiveName(i int) string {
	return fmt.Sprintf("archive-%02d.tar.gz", i)
}

func TestApplier_Apply_MaxAgeDays(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	fresh := newRunWithArchive(t, "job-1", dir, "fresh.tar.gz", now.AddDate(0, 0, -5))
	old := newRunWithArchive(t, "job-1", dir, "old.tar.gz", now.AddDate(0, 0, -40))

	runs := newMemRunStore(fresh, old)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job := testJob(dir, nil, intPtr(30))

	deleted, _, err := a.Apply(context.Background(), job)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := os.Stat(filepath.Join(dir, "old.tar.gz")); !os.IsNotExist(err) {
		t.Error("expected the archive older than max_age_days to be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "fresh.tar.gz")); err != nil {
		t.Error("expected the fresh archive to remain")
	}
}

func TestApplier_Apply_BothLimits_WithinKeepLastIsNeverDeleted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	// 5 archives, 3 of them older than 30 days, but keep_last=10 covers
	// all 5: nothing should be deleted.
	var runsList []*domain.Run
	ages := []int{1, 5, 40, 50, 60}
	for i, age := range ages {
		r := newRunWithArchive(t, "job-1", dir, fmtArchiveName(i), now.AddDate(0, 0, -age))
		runsList = append(runsList, r)
	}
	runs := newMemRunStore(runsList...)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job := testJob(dir, intPtr(10), intPtr(30))

	deleted, _, err := a.Apply(context.Background(), job)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0 (count is within keep_last)", deleted)
	}
}

func TestApplier_Apply_BothLimits_DeletesOnlyOutsideBoth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	// keep_last=2, max_age_days=30, ages ascending (index = recency rank
	// after sorting): rank0/rank1 are within keep_last (kept regardless
	// of age); rank2 is outside keep_last but fresh (kept by max_age);
	// rank3 is outside keep_last AND old (the only deletion).
	ages := []int{1, 2, 5, 40}
	var runsList []*domain.Run
	for i, age := range ages {
		r := newRunWithArchive(t, "job-1", dir, fmtArchiveName(i), now.AddDate(0, 0, -age))
		runsList = append(runsList, r)
	}
	runs := newMemRunStore(runsList...)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job := testJob(dir, intPtr(2), intPtr(30))

	deleted, _, err := a.Apply(context.Background(), job)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := os.Stat(filepath.Join(dir, fmtArchiveName(3))); !os.IsNotExist(err) {
		t.Error("expected the oldest archive (rank 3: outside keep_last and older than max_age_days) to be deleted")
	}
	for _, i := range []int{0, 1, 2} {
		if _, err := os.Stat(filepath.Join(dir, fmtArchiveName(i))); err != nil {
			t.Errorf("expected archive[%d] to remain", i)
		}
	}
}

func TestApplier_Apply_ForeignAndSharedDestinationFilesUntouched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	// A file not produced by any job.
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("write foreign file: %v", err)
	}
	// A second job sharing the same destination directory, also with an
	// old archive that should NOT be touched by job-1's retention run.
	otherJobArchive := newRunWithArchive(t, "job-2", dir, "job2-old.tar.gz", now.AddDate(0, 0, -100))

	job1Old := newRunWithArchive(t, "job-1", dir, "job1-old.tar.gz", now.AddDate(0, 0, -100))
	runs := newMemRunStore(job1Old, otherJobArchive)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job1 := testJob(dir, nil, intPtr(30))

	deleted, _, err := a.Apply(context.Background(), job1)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only job-1's own archive)", deleted)
	}
	if _, err := os.Stat(filepath.Join(dir, "unrelated.txt")); err != nil {
		t.Error("a foreign file must never be deleted by retention")
	}
	if _, err := os.Stat(filepath.Join(dir, "job2-old.tar.gz")); err != nil {
		t.Error("another job's archive in a shared destination must never be deleted by this job's retention run")
	}
}

func TestApplier_Apply_DeletionFailureIsAWarningAndKeepsHistory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	// Reference a file that does not exist on disk to force a delete
	// failure path other than "already gone" (which os.Remove treats as
	// success). Instead, simulate a locked file by making the destination
	// directory read-only after creating the archive, on platforms where
	// that prevents deletion; to stay portable, we instead reference a
	// path that is actually a directory, which os.Remove refuses to
	// remove as a plain file only if non-empty — so create a non-empty
	// subdirectory under the expected archive name.
	archiveName := "locked.tar.gz"
	archiveAsDir := filepath.Join(dir, archiveName)
	if err := os.Mkdir(archiveAsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archiveAsDir, "inner.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write inner file: %v", err)
	}

	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("domain.NewID() error = %v", err)
	}
	run := &domain.Run{ID: id, JobID: "job-1", ArchiveName: archiveName, StartedAt: now.AddDate(0, 0, -100), Status: domain.RunSuccess}

	runs := newMemRunStore(run)
	a := &Applier{Runs: runs, Clock: fakeClock{now}}
	job := testJob(dir, nil, intPtr(30))

	deleted, warnings, err := a.Apply(context.Background(), job)
	if err != nil {
		t.Fatalf("Apply() error = %v (deletion failures must be warnings, not errors)", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	if run.ArchiveName != archiveName {
		t.Error("history record's ArchiveName must be left unchanged when deletion fails")
	}
}
