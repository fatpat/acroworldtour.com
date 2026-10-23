package admin

import (
	"net/http/httptest"
	"testing"
	"time"

	"awt/internal/auth"
)

// The HTTP test cannot pin the minutes (its clock runs); the rounding is
// checked here with fixed durations.
func TestLimitedMessageRoundsUpToMinutes(t *testing.T) {
	for _, c := range []struct {
		retry      time.Duration
		retryAfter string
		msg        string
	}{
		{15 * time.Minute, "900", "Too many wrong passwords, try again in 15 minutes"},
		{14*time.Minute + 1*time.Second, "841", "Too many wrong passwords, try again in 15 minutes"},
		{14 * time.Minute, "840", "Too many wrong passwords, try again in 14 minutes"},
		{500 * time.Millisecond, "1", "Too many wrong passwords, try again in 1 minutes"},
		{0, "1", "Too many wrong passwords, try again in 1 minutes"},
	} {
		w := httptest.NewRecorder()
		if got := limitedMessage(w, &auth.ErrReauthLimited{RetryAfter: c.retry}); got != c.msg {
			t.Errorf("%v: message %q, want %q", c.retry, got, c.msg)
		}
		if got := w.Header().Get("Retry-After"); got != c.retryAfter {
			t.Errorf("%v: Retry-After %q, want %q", c.retry, got, c.retryAfter)
		}
	}
}
