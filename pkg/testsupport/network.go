package testsupport

import (
	"net"
	"testing"
)

// SkipIfCannotListen skips the current test when the sandbox forbids opening local listeners.
func SkipIfCannotListen(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test: cannot open local listener: %v", err)
		return
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
}
