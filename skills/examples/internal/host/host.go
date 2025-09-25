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

// Publish sends a message to the host bus if permitted by the manifest.
func Publish(subject string, payload []byte) bool {
	if len(subject) == 0 {
		return false
	}
	subjectBuf := []byte(subject)
	var payloadPtr unsafe.Pointer
	var payloadLen uint32
	if len(payload) > 0 {
		payloadPtr = unsafe.Pointer(&payload[0])
		payloadLen = uint32(len(payload))
	}
	code := hostPublish(unsafe.Pointer(&subjectBuf[0]), uint32(len(subjectBuf)), payloadPtr, payloadLen)
	return code == 0
}

//go:wasmimport env host_log
func hostLog(ptr unsafe.Pointer, length uint32)

//go:wasmimport env host_publish
func hostPublish(subjectPtr unsafe.Pointer, subjectLen uint32, payloadPtr unsafe.Pointer, payloadLen uint32) uint32
