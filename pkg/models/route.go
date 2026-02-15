package models

import "fmt"

// Route maps a hostname + domain + optional path to an application.
type Route struct {
	GUID    string `json:"guid"`
	AppGUID string `json:"app_guid"`
	Host    string `json:"host"`
	Domain  string `json:"domain"`
	Path    string `json:"path,omitempty"`
}

// URL returns the full route URL.
func (r Route) URL() string {
	url := fmt.Sprintf("%s.%s", r.Host, r.Domain)
	if r.Path != "" {
		url += r.Path
	}
	return url
}
