//go:build !tinygo && !wasm

package host

// Log is a no-op stub for non-wasm builds so that `go test` succeeds.
func Log(string) {}
