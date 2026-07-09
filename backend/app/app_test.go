package app

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

// clearMailfoldEnv unsets every MAILFOLD_ variable a test might have inherited
// from the surrounding shell, so config.Load() sees only what the test itself
// sets via t.Setenv.
func clearMailfoldEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				key := kv[:i]
				if len(key) > 9 && key[:9] == "MAILFOLD_" {
					t.Setenv(key, "")
					_ = os.Unsetenv(key)
				}
				break
			}
		}
	}
}

func TestRunFailsFastOnInvalidConfig(t *testing.T) {
	clearMailfoldEnv(t)
	// None of the required MAILFOLD_MAILCOW_URL / MAILFOLD_MAILCOW_API_KEY /
	// MAILFOLD_ADMIN_PASSWORD are set, so Run must fail during config.Load
	// without ever attempting to open a listener.
	err := Run()
	if err == nil {
		t.Fatal("Run() with no configuration should return an error")
	}
}

// TestRunServesAndShutsDownGracefully exercises the real production entry
// point end to end: it boots Run() against a minimal valid configuration,
// waits for the HTTP server to actually accept connections, sends the
// process a real SIGTERM (exactly what a container runtime sends on
// `docker stop`), and asserts Run() unwinds through its graceful-shutdown
// path and returns nil rather than an error.
func TestRunServesAndShutsDownGracefully(t *testing.T) {
	clearMailfoldEnv(t)
	const addr = "127.0.0.1:18734"
	t.Setenv("MAILFOLD_ADDR", addr)
	t.Setenv("MAILFOLD_MAILCOW_URL", "http://127.0.0.1:1")
	t.Setenv("MAILFOLD_MAILCOW_API_KEY", "test-key")
	t.Setenv("MAILFOLD_ADMIN_PASSWORD", "test-password-not-used")

	done := make(chan error, 1)
	go func() { done <- Run() }()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/api/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				lastErr = nil
				break
			}
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("server never became healthy: %v", lastErr)
	}

	// signal.Notify in Run is registered synchronously, on the same
	// goroutine, immediately after the listener goroutine is spawned — well
	// before that goroutine can possibly finish net.Listen and start serving
	// requests. Since the health check above already succeeded, Run is
	// certainly past signal.Notify and blocked in its shutdown select.
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("sending SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned %v after SIGTERM, want a clean shutdown (nil)", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not return within 15s of SIGTERM")
	}
}
