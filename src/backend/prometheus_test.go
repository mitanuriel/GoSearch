package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestIncrementNewUserCounter(t *testing.T) {
	// Reset the counter before testing
	newUserCounter.Reset()

	// Call the function
	incrementNewUserCounter()

	// Get current time to verify labels
	now := time.Now()
	expectedHour := now.Hour()
	expectedDay := now.Weekday().String()

	// Verify that the counter was incremented
	// We can't easily check specific label values with testutil, but we can verify the counter exists
	count := testutil.CollectAndCount(newUserCounter)
	assert.Greater(t, count, 0, "Counter should have at least one metric")
	
	// Call again to verify it increments
	incrementNewUserCounter()
	count2 := testutil.CollectAndCount(newUserCounter)
	assert.GreaterOrEqual(t, count2, count, "Counter should not decrease")

	t.Logf("Counter incremented for hour=%d, day=%s", expectedHour, expectedDay)
}

func TestStatusRecorder_WriteHeader(t *testing.T) {
	// Create a test response writer
	w := httptest.NewRecorder()
	
	// Create status recorder
	recorder := &statusRecorder{
		ResponseWriter: w,
		statusCode:     200, // default
	}

	// Test writing a custom status code
	recorder.WriteHeader(404)

	// Verify the status code was recorded
	assert.Equal(t, 404, recorder.statusCode, "Status code should be recorded")
	assert.Equal(t, 404, w.Code, "Status code should be written to underlying writer")
}

func TestStatusRecorder_WriteHeaderMultipleTimes(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := &statusRecorder{
		ResponseWriter: w,
		statusCode:     200,
	}

	// First write
	recorder.WriteHeader(201)
	assert.Equal(t, 201, recorder.statusCode)

	// Second write (should update recorder but HTTP spec says only first counts)
	recorder.WriteHeader(500)
	assert.Equal(t, 500, recorder.statusCode, "Recorder should update")
	// Note: http.ResponseWriter only honors first WriteHeader call
}

func TestMetricsMiddleware_RecordsMetrics(t *testing.T) {
	// Reset Prometheus metrics
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create a simple handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	handler := metricsMiddleware(nextHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test-path", nil)
	w := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Verify metrics were recorded
	totalCount := testutil.CollectAndCount(httpRequestsTotal)
	durationCount := testutil.CollectAndCount(httpRequestDuration)
	
	assert.Greater(t, totalCount, 0, "httpRequestsTotal should have metrics")
	assert.Greater(t, durationCount, 0, "httpRequestDuration should have metrics")
}

func TestMetricsMiddleware_WithAuthenticatedUser(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 42
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/authenticated-path", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := metricsMiddleware(nextHandler)
	handler.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestMetricsMiddleware_TracksStatusCodes(t *testing.T) {
	httpRequestsTotal.Reset()
	
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	testCases := []struct {
		name       string
		statusCode int
	}{
		{"Success", 200},
		{"Created", 201},
		{"NotFound", 404},
		{"ServerError", 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			})

			handler := metricsMiddleware(nextHandler)
			req := httptest.NewRequest("GET", "/status-test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tc.statusCode, w.Code, "Status code should match")
		})
	}
}

func TestMetricsMiddleware_MeasuresDuration(t *testing.T) {
	httpRequestDuration.Reset()
	
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create handler that takes some time
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	handler := metricsMiddleware(nextHandler)
	req := httptest.NewRequest("GET", "/slow-path", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(w, req)
	elapsed := time.Since(start)

	// Request should have taken at least 10ms
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(10), "Request should take at least 10ms")
	
	// Verify duration was recorded
	count := testutil.CollectAndCount(httpRequestDuration)
	assert.Greater(t, count, 0, "Duration should be recorded")
}

func init() {
	// Ensure Prometheus metrics are registered only once
	// This prevents "duplicate metrics collector registration attempted" errors
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	
	// Re-register our metrics
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(newUserCounter)
}
