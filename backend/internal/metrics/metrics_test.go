package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestObserveAndRender(t *testing.T) {
	m := New()
	m.Observe("GET", 200, 20*time.Millisecond)
	m.Observe("GET", 200, 300*time.Millisecond)
	m.Observe("POST", 401, 5*time.Millisecond)

	var sb strings.Builder
	m.WritePrometheus(&sb)
	out := sb.String()

	for _, want := range []string{
		`mailfold_http_requests_total{method="GET",code="200"} 2`,
		`mailfold_http_requests_total{method="POST",code="401"} 1`,
		"# TYPE mailfold_http_requests_total counter",
		`mailfold_http_request_duration_seconds_bucket{le="+Inf"} 3`,
		"mailfold_http_request_duration_seconds_count 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n---\n%s", want, out)
		}
	}
}

func TestEmptyRender(t *testing.T) {
	var sb strings.Builder
	New().WritePrometheus(&sb)
	if !strings.Contains(sb.String(), "mailfold_http_requests_total") {
		t.Error("expected HELP/TYPE lines even with no observations")
	}
}
