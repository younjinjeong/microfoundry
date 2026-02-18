package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AlertmanagerClient queries Alertmanager for active alerts.
type AlertmanagerClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Alert represents an active alert from Alertmanager.
type Alert struct {
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Severity    string            `json:"severity"`
	Summary     string            `json:"summary"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

// ListAlerts returns all active alerts from Alertmanager.
func (a *AlertmanagerClient) ListAlerts(ctx context.Context, filters ...string) ([]Alert, error) {
	u, err := url.Parse(a.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing alertmanager URL: %w", err)
	}
	u.Path = "/api/v2/alerts"

	q := u.Query()
	for _, f := range filters {
		q.Add("filter", f)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying alertmanager: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alertmanager returned %d: %s", resp.StatusCode, string(body))
	}

	var amAlerts []amAlert
	if err := json.NewDecoder(resp.Body).Decode(&amAlerts); err != nil {
		return nil, fmt.Errorf("decoding alertmanager response: %w", err)
	}

	return convertAlerts(amAlerts), nil
}

// amAlert is the Alertmanager v2 API alert format.
type amAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	Status      struct {
		State string `json:"state"` // "active", "suppressed", "unprocessed"
	} `json:"status"`
}

// HealthCheck verifies that Alertmanager is reachable.
func (a *AlertmanagerClient) HealthCheck(ctx context.Context) error {
	u, err := url.Parse(a.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid alertmanager URL: %w", err)
	}
	u.Path = "/-/healthy"

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("alertmanager unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alertmanager returned %s", resp.Status)
	}
	return nil
}

func convertAlerts(amAlerts []amAlert) []Alert {
	alerts := make([]Alert, 0, len(amAlerts))
	for _, a := range amAlerts {
		alert := Alert{
			Name:        a.Labels["alertname"],
			Status:      a.Status.State,
			Severity:    a.Labels["severity"],
			Summary:     a.Annotations["summary"],
			Labels:      a.Labels,
			Annotations: a.Annotations,
			StartsAt:    a.StartsAt,
			EndsAt:      a.EndsAt,
		}
		if alert.Status == "active" {
			alert.Status = "firing"
		}
		alerts = append(alerts, alert)
	}
	return alerts
}
