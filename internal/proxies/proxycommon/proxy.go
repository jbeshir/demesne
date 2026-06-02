package proxycommon

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
)

// Serve binds bindAddr and serves srv until ctx is cancelled, then
// gracefully shuts it down (5s drain). Returns nil on clean shutdown;
// other errors are surfaced. logName prefixes the listen/shutdown log
// lines (e.g. "anthropic proxy").
func Serve(ctx context.Context, bindAddr string, srv *http.Server, logName string) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", bindAddr)
	if err != nil {
		return err
	}
	log.Printf("%s: listening on %s", logName, ln.Addr())

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		// WithoutCancel preserves values/deadlines from ctx but drops
		// the cancellation — Shutdown needs a live ctx to drain.
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Printf("%s: shutdown error: %v", logName, err)
		}
		close(done)
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-done
	return nil
}

// Deny logs a rejected request and writes an HTTP error. The method and
// path are %q-escaped so attacker-controlled values can't inject fake
// log lines. logName prefixes the log line (e.g. "anthropic proxy").
func Deny(w http.ResponseWriter, r *http.Request, code int, reason, logName string) {
	//nolint:gosec // G706: values are %q-escaped, defeating log injection
	log.Printf("%s: deny method=%q path=%q reason=%s code=%d", logName, r.Method, r.URL.Path, reason, code)
	http.Error(w, reason, code)
}
