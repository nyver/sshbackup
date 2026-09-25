package backup

import (
	"testing"

	"vpsbackupmanager/internal/domain"
)

func TestArchiveCommand(t *testing.T) {
	t.Parallel()

	const dir = "'/tmp/vpsbackup/job1'"
	tests := []struct {
		name    string
		sources []domain.Source
		want    string
	}{
		{
			name: "no include: plain tar over the sources",
			sources: []domain.Source{
				{RemotePath: "/docker/volumes", Exclude: []string{"*.log"}},
				{RemotePath: "/etc/app"},
			},
			want: "mkdir -p " + dir + " && tar --exclude='*.log' -czf '/tmp/vpsbackup/job1/a.tar.gz' -- '/docker/volumes' '/etc/app'",
		},
		{
			name: "include: entries are listed with find and read by tar",
			sources: []domain.Source{
				{RemotePath: "/docker/volumes", Include: []string{"*.sql", "data/"}, Exclude: []string{"cache"}},
			},
			want: "mkdir -p " + dir + " && { find '/docker/volumes' -mindepth 1 \\( -name '*.sql' -o -name 'data' \\) -prune -print0; } > '/tmp/vpsbackup/job1/a.tar.gz.list'" +
				" && tar --null -T '/tmp/vpsbackup/job1/a.tar.gz.list' --exclude='cache' -czf '/tmp/vpsbackup/job1/a.tar.gz'" +
				"; status=$?; rm -f '/tmp/vpsbackup/job1/a.tar.gz.list'; exit $status",
		},
		{
			name: "include on one source only: the other is archived whole",
			sources: []domain.Source{
				{RemotePath: "/a", Include: []string{"it's"}},
				{RemotePath: "/b"},
			},
			want: "mkdir -p " + dir + " && { find '/a' -mindepth 1 \\( -name 'it'\\''s' \\) -prune -print0 && printf '%s\\000' '/b'; } > '/tmp/vpsbackup/job1/a.tar.gz.list'" +
				" && tar --null -T '/tmp/vpsbackup/job1/a.tar.gz.list' -czf '/tmp/vpsbackup/job1/a.tar.gz'" +
				"; status=$?; rm -f '/tmp/vpsbackup/job1/a.tar.gz.list'; exit $status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc := &runContext{
				job: &domain.Job{
					ID:                  "job1",
					RemoteTempDirectory: "/tmp/vpsbackup",
					Sources:             tt.sources,
				},
				remoteArchivePath: "/tmp/vpsbackup/job1/a.tar.gz",
			}
			if got := rc.archiveCommand(); got != tt.want {
				t.Errorf("archiveCommand() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
