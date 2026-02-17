package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidAppName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"simple", "hello", true},
		{"with-hyphen", "hello-world", true},
		{"with-numbers", "app123", true},
		{"mixed", "my-app-v2", true},
		{"single-char", "a", true},
		{"starts-with-number", "1app", true},

		// PromQL injection attempts
		{"injection-curly", `app{job="x"}`, false},
		{"injection-paren", "app()", false},
		{"injection-quote", `app"test`, false},
		{"injection-backslash", `app\n`, false},
		{"injection-semicolon", "app;drop", false},
		{"injection-pipe", "app|other", false},
		{"injection-space", "app name", false},
		{"injection-newline", "app\nother", false},
		{"uppercase", "MyApp", false},
		{"starts-with-hyphen", "-app", false},
		{"ends-with-hyphen", "app-", false},
		{"empty", "", false},
		{"dot-path", "../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validAppName.MatchString(tt.input)
			if got != tt.valid {
				t.Errorf("validAppName.MatchString(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestGetAppREDMetrics_InvalidName(t *testing.T) {
	client := NewPrometheusClient("http://localhost:9090")
	_, err := client.GetAppREDMetrics(context.Background(), `evil{job="x"}`)
	if err == nil {
		t.Fatal("expected error for invalid app name, got nil")
	}
}

func TestGetAppREDMetrics_Success(t *testing.T) {
	// Mock Prometheus server that returns a valid instant query response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := prometheusResponse{
			Status: "success",
		}
		resp.Data.ResultType = "vector"
		resp.Data.Result = []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
		}{
			{
				Metric: map[string]string{"k8s_deployment_name": "test-app"},
				Value:  [2]any{1234567890.0, "0.42"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewPrometheusClient(ts.URL)
	metrics, err := client.GetAppREDMetrics(context.Background(), "test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !metrics.HasData {
		t.Error("expected HasData to be true")
	}
	if metrics.RequestRate != 0.42 {
		t.Errorf("expected RequestRate=0.42, got %v", metrics.RequestRate)
	}
}

func TestGetAppREDMetrics_NoData(t *testing.T) {
	// Mock Prometheus server that returns empty results
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := prometheusResponse{
			Status: "success",
		}
		resp.Data.ResultType = "vector"
		resp.Data.Result = nil
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewPrometheusClient(ts.URL)
	metrics, err := client.GetAppREDMetrics(context.Background(), "test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.HasData {
		t.Error("expected HasData to be false for empty results")
	}
}

func TestGetAppREDMetrics_PartialFailure(t *testing.T) {
	// Mock server that alternates between success and error
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := prometheusResponse{
			Status: "success",
		}
		resp.Data.ResultType = "vector"
		resp.Data.Result = []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
		}{
			{
				Metric: map[string]string{},
				Value:  [2]any{1234567890.0, "1.5"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewPrometheusClient(ts.URL)
	metrics, err := client.GetAppREDMetrics(context.Background(), "test-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have partial data and warnings
	if len(metrics.Warnings) == 0 {
		t.Error("expected warnings for partial failures")
	}
}

func TestInstantQuery_ServerDown(t *testing.T) {
	client := NewPrometheusClient("http://127.0.0.1:1") // unreachable
	_, err := client.instantQuery(context.Background(), "up")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
