package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// validAppName validates app names to prevent PromQL injection.
// Only allows lowercase alphanumeric characters and hyphens.
var validAppName = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

// PrometheusClient queries Prometheus for pre-computed recording rule values.
type PrometheusClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// REDMetrics holds Rate, Error rate, and Duration percentiles for an app.
type REDMetrics struct {
	RequestRate float64  `json:"request_rate"`
	ErrorRate   float64  `json:"error_rate"`
	LatencyP50  float64  `json:"latency_p50"`
	LatencyP95  float64  `json:"latency_p95"`
	LatencyP99  float64  `json:"latency_p99"`
	HasData     bool     `json:"has_data"`
	Warnings    []string `json:"warnings,omitempty"`
}

// prometheusResponse is the JSON envelope from the Prometheus instant query API.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"` // [timestamp, "value"]
		} `json:"result"`
	} `json:"data"`
}

// GetAppREDMetrics queries Prometheus for pre-computed RED metrics for an app.
// Queries run in parallel using errgroup for reduced latency.
func (p *PrometheusClient) GetAppREDMetrics(ctx context.Context, appName string) (*REDMetrics, error) {
	if !validAppName.MatchString(appName) {
		return nil, fmt.Errorf("invalid app name: %q", appName)
	}

	// Apply query timeout as circuit breaker
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	metrics := &REDMetrics{}
	var mu sync.Mutex
	var warnings []string

	type queryTarget struct {
		query string
		dest  *float64
		name  string
	}

	targets := []queryTarget{
		{fmt.Sprintf(`microfoundry:http_request_rate:5m{k8s_deployment_name="%s"}`, appName), &metrics.RequestRate, "request_rate"},
		{fmt.Sprintf(`microfoundry:http_error_rate:5m{k8s_deployment_name="%s"}`, appName), &metrics.ErrorRate, "error_rate"},
		{fmt.Sprintf(`microfoundry:http_latency_p50:5m{k8s_deployment_name="%s"}`, appName), &metrics.LatencyP50, "latency_p50"},
		{fmt.Sprintf(`microfoundry:http_latency_p95:5m{k8s_deployment_name="%s"}`, appName), &metrics.LatencyP95, "latency_p95"},
		{fmt.Sprintf(`microfoundry:http_latency_p99:5m{k8s_deployment_name="%s"}`, appName), &metrics.LatencyP99, "latency_p99"},
	}

	g, ctx := errgroup.WithContext(ctx)

	for _, t := range targets {
		t := t // capture loop var
		g.Go(func() error {
			val, err := p.instantQuery(ctx, t.query)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", t.name, err))
				return nil // don't fail the group for partial data
			}
			if !math.IsNaN(val) {
				*t.dest = val
				metrics.HasData = true
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return metrics, fmt.Errorf("prometheus queries failed: %w", err)
	}

	if len(warnings) > 0 {
		metrics.Warnings = warnings
	}

	return metrics, nil
}

// instantQuery executes a Prometheus instant query and returns the scalar value.
func (p *PrometheusClient) instantQuery(ctx context.Context, query string) (float64, error) {
	u, err := url.Parse(p.BaseURL)
	if err != nil {
		return 0, err
	}
	u.Path = "/api/v1/query"
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus query failed: %s", resp.Status)
	}

	var pr prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return 0, err
	}

	if pr.Status != "success" || len(pr.Data.Result) == 0 {
		return math.NaN(), nil
	}

	valStr, ok := pr.Data.Result[0].Value[1].(string)
	if !ok {
		return math.NaN(), nil
	}
	return strconv.ParseFloat(valStr, 64)
}

// NewPrometheusClient creates a Prometheus query client.
func NewPrometheusClient(baseURL string) *PrometheusClient {
	return &PrometheusClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}
