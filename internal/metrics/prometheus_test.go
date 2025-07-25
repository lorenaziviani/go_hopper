package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrometheusMetrics(t *testing.T) {
	metrics := NewPrometheusMetrics()
	if metrics == nil {
		t.Fatal("Failed to create Prometheus metrics")
	}

	metrics.IncrementMessagesProcessed()
	metrics.IncrementMessagesProcessed()
	metrics.IncrementMessagesFailed()
	metrics.IncrementRetryAttempts()

	metrics.SetActiveWorkers(5)
	metrics.SetQueueSize(10)

	metrics.ObserveProcessingDuration(100 * time.Millisecond)
	metrics.ObserveProcessingDuration(200 * time.Millisecond)

	handler := metrics.GetHandler()
	if handler == nil {
		t.Fatal("Failed to get HTTP handler")
	}

	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	body := rr.Body.String()
	expectedMetrics := []string{
		"messages_processed_total",
		"messages_failed_total",
		"retry_attempts_total",
		"processing_duration_seconds",
		"active_workers",
		"queue_size",
	}

	for _, metric := range expectedMetrics {
		if !contains(body, metric) {
			t.Errorf("Response body does not contain metric: %s", metric)
		}
	}

	t.Log("Prometheus metrics test passed successfully")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
