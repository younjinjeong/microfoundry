package models

import "fmt"

// Route protocol constants.
const (
	ProtocolHTTP      = ""          // default — plain HTTP/HTTPS
	ProtocolWebSocket = "websocket" // WebSocket (ws/wss)
	ProtocolGRPC      = "grpc"      // gRPC (HTTP/2)
	ProtocolTCP       = "tcp"       // raw TCP (future — Gateway API)
)

// Route maps a hostname + domain + optional path to an application.
type Route struct {
	GUID     string `json:"guid"`
	AppGUID  string `json:"app_guid"`
	Host     string `json:"host"`
	Domain   string `json:"domain"`
	Path     string `json:"path,omitempty"`
	Protocol string `json:"protocol,omitempty"` // "", "websocket", "grpc", "tcp"
}

// URL returns the full route URL.
func (r Route) URL() string {
	url := fmt.Sprintf("%s.%s", r.Host, r.Domain)
	if r.Path != "" {
		url += r.Path
	}
	return url
}

// Scheme returns the URL scheme for this route given a TLS context.
func (r Route) Scheme(hasTLS bool) string {
	switch r.Protocol {
	case ProtocolWebSocket:
		if hasTLS {
			return "wss"
		}
		return "ws"
	case ProtocolGRPC:
		return "grpcs" // gRPC always over TLS in production
	default:
		if hasTLS {
			return "https"
		}
		return "http"
	}
}

// IsWebSocket returns true if this route uses the WebSocket protocol.
func (r Route) IsWebSocket() bool {
	return r.Protocol == ProtocolWebSocket
}

// IsGRPC returns true if this route uses the gRPC protocol.
func (r Route) IsGRPC() bool {
	return r.Protocol == ProtocolGRPC
}

// HasSpecialProtocol returns true if the route uses a non-HTTP protocol.
func HasSpecialProtocol(routes []Route) string {
	for _, r := range routes {
		if r.Protocol != "" {
			return r.Protocol
		}
	}
	return ""
}
