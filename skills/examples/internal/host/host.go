//go:build tinygo || wasm

package host

import "unsafe"

// Log forwards text to the host runtime via the imported host_log function.
func Log(msg string) {
	if len(msg) == 0 {
		return
	}
	b := []byte(msg)
	hostLog(unsafe.Pointer(&b[0]), uint32(len(b)))
}

//go:wasmimport env host_log
func hostLog(ptr unsafe.Pointer, length uint32)
