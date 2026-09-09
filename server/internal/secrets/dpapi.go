// Package secrets protects SSH passphrases and other credential material
// with Windows DPAPI, machine-scoped so the service account can decrypt
// them regardless of which interactive session created them. Plaintext
// secrets never reach SQLite: callers store only the opaque reference this
// package returns.
package secrets

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modCrypt32  = windows.NewLazySystemDLL("crypt32.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
	procLocalFree          = modKernel32.NewProc("LocalFree")
)

// cryptProtectLocalMachine is CRYPTPROTECT_LOCAL_MACHINE: protect at
// machine scope rather than the calling user's profile, since the Windows
// Service runs under its own service account.
const cryptProtectLocalMachine = 0x4

// dataBlob mirrors the Win32 CRYPT_INTEGER_BLOB / DATA_BLOB structure.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(b []byte) *dataBlob {
	if len(b) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(b)), pbData: &b[0]} //nolint:gosec // b is a credential value, always far below 4 GiB
}

func (b *dataBlob) bytes() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	return unsafe.Slice(b.pbData, int(b.cbData)) //nolint:gosec // required to view the DPAPI-owned output buffer as a Go slice
}

// dpapiProtect encrypts data with CryptProtectData at machine scope.
func dpapiProtect(data []byte) ([]byte, error) {
	out, err := callDPAPI(procCryptProtectData, data)
	if err != nil {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	return out, nil
}

// dpapiUnprotect decrypts data previously produced by dpapiProtect.
func dpapiUnprotect(data []byte) ([]byte, error) {
	out, err := callDPAPI(procCryptUnprotectData, data)
	if err != nil {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	return out, nil
}

// callDPAPI invokes proc, which must be CryptProtectData or
// CryptUnprotectData: both share the same (in, descr, entropy, reserved,
// promptStruct, flags, out) signature and BOOL/DATA_BLOB* out-parameter
// convention.
func callDPAPI(proc *windows.LazyProc, data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	ret, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(in)), //nolint:gosec // in is kept alive by this frame for the duration of the syscall
		0,                           // szDataDescr
		0,                           // pOptionalEntropy
		0,                           // pvReserved
		0,                           // pPromptStruct
		uintptr(cryptProtectLocalMachine),
		uintptr(unsafe.Pointer(&out)), //nolint:gosec // out is kept alive by this frame for the duration of the syscall
	)
	if ret == 0 {
		return nil, callErr
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData))) //nolint:errcheck,gosec // best-effort free of the DPAPI-allocated output buffer

	result := make([]byte, out.cbData)
	copy(result, out.bytes())
	return result, nil
}
