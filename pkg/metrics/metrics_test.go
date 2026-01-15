package metrics

import (
    "testing"
    "github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPRequestsTotal(t *testing.T) {
    // Reset counter before test
    HTTPRequestsTotal.Reset()

    // Increment counter
    HTTPRequestsTotal.WithLabelValues("GET", "/api/status", "200").Inc()
    HTTPRequestsTotal.WithLabelValues("GET", "/api/status", "200").Inc()
    HTTPRequestsTotal.WithLabelValues("POST", "/api/jobs", "201").Inc()

    // Verify counts
    count := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/api/status", "200"))
    if count != 2 {
        t.Errorf("Expected 2 GET requests, got %f", count)
    }

    count = testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("POST", "/api/jobs", "201"))
    if count != 1 {
        t.Errorf("Expected 1 POST request, got %f", count)
    }
}
