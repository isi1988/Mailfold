// Package metrics provides a tiny, dependency-free HTTP metrics collector that
// exposes counters and a latency histogram in Prometheus text exposition
// format. It is deliberately minimal: rather than pulling in the Prometheus
// client library, it records exactly the few series a small admin backend needs
// and renders them on demand.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// buckets are the upper bounds (in seconds) of the latency histogram. They are
// the conventional Prometheus default buckets, suitable for typical web latency.
var buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// Metrics accumulates HTTP request counters and a latency histogram. It is safe
// for concurrent use.
type Metrics struct {
	mu sync.Mutex

	// reqCount counts requests keyed by "method|code".
	reqCount map[string]uint64

	// bucketCounts[i] holds the cumulative number of observations whose duration
	// was <= buckets[i]; sumSeconds and total back the histogram's _sum/_count.
	bucketCounts []uint64
	sumSeconds   float64
	total        uint64
}

// New creates an empty Metrics collector.
func New() *Metrics {
	return &Metrics{
		reqCount:     make(map[string]uint64),
		bucketCounts: make([]uint64, len(buckets)),
	}
}

// Observe records a single completed request: its method, response status code,
// and how long it took.
func (m *Metrics) Observe(method string, code int, d time.Duration) {
	secs := d.Seconds()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.reqCount[method+"|"+strconv.Itoa(code)]++
	m.total++
	m.sumSeconds += secs
	for i, upper := range buckets {
		if secs <= upper {
			m.bucketCounts[i]++
		}
	}
}

// WritePrometheus renders the collected metrics in Prometheus text format.
func (m *Metrics) WritePrometheus(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// p writes a formatted line, deliberately ignoring the write error: metrics
	// scraping is best-effort and there is nothing useful to do if the scraper
	// disconnects mid-response.
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }

	p("# HELP mailfold_http_requests_total Total number of HTTP requests.\n")
	p("# TYPE mailfold_http_requests_total counter\n")
	keys := make([]string, 0, len(m.reqCount))
	for k := range m.reqCount {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		method, code, _ := strings.Cut(k, "|")
		p("mailfold_http_requests_total{method=%q,code=%q} %d\n", method, code, m.reqCount[k])
	}

	p("# HELP mailfold_http_request_duration_seconds HTTP request latency.\n")
	p("# TYPE mailfold_http_request_duration_seconds histogram\n")
	for i, upper := range buckets {
		p("mailfold_http_request_duration_seconds_bucket{le=%q} %d\n",
			strconv.FormatFloat(upper, 'g', -1, 64), m.bucketCounts[i])
	}
	p("mailfold_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", m.total)
	p("mailfold_http_request_duration_seconds_sum %g\n", m.sumSeconds)
	p("mailfold_http_request_duration_seconds_count %d\n", m.total)
}
