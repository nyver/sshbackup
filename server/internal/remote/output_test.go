package remote

import "testing"

func TestCappedWriter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		limit         int
		writes        []string
		wantString    string
		wantTruncated bool
	}{
		{"under limit", 10, []string{"hello"}, "hello", false},
		{"exactly at limit", 5, []string{"hello"}, "hello", false},
		{"single write over limit", 5, []string{"hello world"}, "hello", true},
		{"multiple writes crossing limit", 5, []string{"he", "llo", "world"}, "hello", true},
		{"write after already full", 3, []string{"abc", "def"}, "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := newCappedWriter(tt.limit)
			for _, chunk := range tt.writes {
				n, err := w.Write([]byte(chunk))
				if err != nil {
					t.Fatalf("Write(%q) error = %v", chunk, err)
				}
				if n != len(chunk) {
					t.Errorf("Write(%q) returned n=%d, want %d", chunk, n, len(chunk))
				}
			}
			if got := w.String(); got != tt.wantString {
				t.Errorf("String() = %q, want %q", got, tt.wantString)
			}
			if w.truncated != tt.wantTruncated {
				t.Errorf("truncated = %v, want %v", w.truncated, tt.wantTruncated)
			}
		})
	}
}
