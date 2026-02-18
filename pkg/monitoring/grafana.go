package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GrafanaConfig holds the connection info for embedded Grafana.
type GrafanaConfig struct {
	BaseURL string
}

// HealthCheck verifies that Grafana is reachable.
func (g *GrafanaConfig) HealthCheck(ctx context.Context) error {
	u, err := url.Parse(g.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid grafana URL: %w", err)
	}
	u.Path = "/api/health"

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("grafana unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana returned %s", resp.Status)
	}
	return nil
}

// DashboardURL returns an embeddable Grafana solo dashboard URL.
func (g *GrafanaConfig) DashboardURL(uid string, params map[string]string) string {
	u, _ := url.Parse(g.BaseURL)
	u.Path = fmt.Sprintf("/d-solo/%s", uid)
	q := u.Query()
	q.Set("orgId", "1")
	q.Set("theme", "light")
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// PanelURL returns an embeddable URL for a specific panel within a dashboard.
func (g *GrafanaConfig) PanelURL(uid string, panelID int, params map[string]string) string {
	u, _ := url.Parse(g.BaseURL)
	u.Path = fmt.Sprintf("/d-solo/%s", uid)
	q := u.Query()
	q.Set("orgId", "1")
	q.Set("theme", "light")
	q.Set("panelId", fmt.Sprintf("%d", panelID))
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// FullDashboardURL returns a non-solo Grafana dashboard URL for "Open in Grafana" links.
func (g *GrafanaConfig) FullDashboardURL(uid string, params map[string]string) string {
	u, _ := url.Parse(g.BaseURL)
	u.Path = fmt.Sprintf("/d/%s", uid)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
