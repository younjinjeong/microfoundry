package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// LokiClient queries Loki for historical log entries.
type LokiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// LogEntry represents a single log line from Loki.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Line      string            `json:"line"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// QueryLogs queries Loki for logs matching the given namespace and app.
func (l *LokiClient) QueryLogs(ctx context.Context, namespace, app string, start, end time.Time, limit int) ([]LogEntry, error) {
	query := fmt.Sprintf(`{namespace="%s", app="%s"}`, namespace, app)
	return l.queryRange(ctx, query, start, end, limit)
}

// QueryAllLogs queries Loki for all logs in a namespace.
func (l *LokiClient) QueryAllLogs(ctx context.Context, namespace string, start, end time.Time, limit int) ([]LogEntry, error) {
	query := fmt.Sprintf(`{namespace="%s"}`, namespace)
	return l.queryRange(ctx, query, start, end, limit)
}

func (l *LokiClient) queryRange(ctx context.Context, query string, start, end time.Time, limit int) ([]LogEntry, error) {
	u, err := url.Parse(l.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing loki URL: %w", err)
	}
	u.Path = "/loki/api/v1/query_range"

	q := u.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	q.Set("end", fmt.Sprintf("%d", end.UnixNano()))
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("direction", "backward")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki returned %d: %s", resp.StatusCode, string(body))
	}

	var lokiResp lokiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&lokiResp); err != nil {
		return nil, fmt.Errorf("decoding loki response: %w", err)
	}

	return parseLokiStreams(lokiResp), nil
}

// HealthCheck verifies that Loki is reachable.
func (l *LokiClient) HealthCheck(ctx context.Context) error {
	u, err := url.Parse(l.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid loki URL: %w", err)
	}
	u.Path = "/ready"

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("loki unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki returned %s", resp.Status)
	}
	return nil
}

type lokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []lokiStream `json:"result"`
	} `json:"data"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"` // [timestamp_ns, line]
}

func parseLokiStreams(resp lokiQueryResponse) []LogEntry {
	var entries []LogEntry
	for _, stream := range resp.Data.Result {
		for _, val := range stream.Values {
			if len(val) < 2 {
				continue
			}
			ts, err := strconv.ParseInt(val[0], 10, 64)
			if err != nil {
				continue
			}
			entries = append(entries, LogEntry{
				Timestamp: time.Unix(0, ts),
				Line:      val[1],
				Labels:    stream.Stream,
			})
		}
	}

	// Sort by timestamp ascending for display
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries
}
