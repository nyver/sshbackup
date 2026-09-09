package secrets

import (
	"bytes"
	"runtime"
	"testing"
)

func TestDPAPI_ProtectUnprotectRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI is only available on Windows")
	}
	t.Parallel()

	plaintext := []byte("correct horse battery staple")
	protected, err := dpapiProtect(plaintext)
	if err != nil {
		t.Fatalf("dpapiProtect() error = %v", err)
	}
	if bytes.Equal(protected, plaintext) {
		t.Fatal("protected output must not equal the plaintext")
	}

	got, err := dpapiUnprotect(protected)
	if err != nil {
		t.Fatalf("dpapiUnprotect() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round trip = %q, want %q", got, plaintext)
	}
}

func TestDPAPI_UnprotectGarbageFails(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI is only available on Windows")
	}
	t.Parallel()

	if _, err := dpapiUnprotect([]byte("not a real DPAPI blob")); err == nil {
		t.Fatal("expected dpapiUnprotect to fail on garbage input")
	}
}
