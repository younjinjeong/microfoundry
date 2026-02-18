package k8s

// GatewayProvider encapsulates ingress-controller-specific configuration.
// Each provider returns the correct IngressClass name and annotation set
// for its gateway type (nginx, kong, traefik, etc.).
type GatewayProvider interface {
	// IngressClass returns the Kubernetes IngressClass name (e.g., "nginx", "kong").
	IngressClass() string

	// AppAnnotations returns annotations for application ingresses.
	AppAnnotations() map[string]string

	// SystemAnnotations returns annotations for system-service ingresses (Keycloak, Grafana, etc.).
	// serviceName identifies the service so providers can add service-specific annotations.
	SystemAnnotations(serviceName string) map[string]string

	// ProtocolAnnotations returns annotations required for a specific protocol.
	// protocol is one of the models.Protocol* constants ("websocket", "grpc", "tcp").
	// Returns nil for standard HTTP traffic.
	ProtocolAnnotations(protocol string) map[string]string
}

// NewGatewayProvider creates the appropriate provider for the given ingress class.
// Unrecognised values default to nginx for backward compatibility.
func NewGatewayProvider(ingressClass string) GatewayProvider {
	switch ingressClass {
	case "kong":
		return &kongProvider{}
	case "traefik":
		return &traefikProvider{}
	default:
		return &nginxProvider{}
	}
}

// ---------------------------------------------------------------------------
// nginx (default)
// ---------------------------------------------------------------------------

type nginxProvider struct{}

func (p *nginxProvider) IngressClass() string { return "nginx" }

func (p *nginxProvider) AppAnnotations() map[string]string {
	return map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/",
	}
}

func (p *nginxProvider) SystemAnnotations(serviceName string) map[string]string {
	ann := map[string]string{}
	if serviceName == keycloakName {
		ann["nginx.ingress.kubernetes.io/proxy-buffer-size"] = "128k"
	}
	return ann
}

func (p *nginxProvider) ProtocolAnnotations(protocol string) map[string]string {
	switch protocol {
	case "websocket":
		return map[string]string{
			"nginx.ingress.kubernetes.io/proxy-read-timeout":  "3600",
			"nginx.ingress.kubernetes.io/proxy-send-timeout":  "3600",
			"nginx.ingress.kubernetes.io/proxy-http-version":  "1.1",
			"nginx.ingress.kubernetes.io/connection-proxy-header": "upgrade",
		}
	case "grpc":
		return map[string]string{
			"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
		}
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Kong
// ---------------------------------------------------------------------------

type kongProvider struct{}

func (p *kongProvider) IngressClass() string { return "kong" }

func (p *kongProvider) AppAnnotations() map[string]string {
	return map[string]string{
		"konghq.com/strip-path": "true",
	}
}

func (p *kongProvider) SystemAnnotations(serviceName string) map[string]string {
	ann := map[string]string{}
	if serviceName == keycloakName {
		ann["konghq.com/proxy-buffering"] = "on"
	}
	return ann
}

func (p *kongProvider) ProtocolAnnotations(protocol string) map[string]string {
	switch protocol {
	case "websocket":
		return map[string]string{
			"konghq.com/protocols":    "ws,wss",
			"konghq.com/read-timeout": "3600000",
		}
	case "grpc":
		return map[string]string{
			"konghq.com/protocols": "grpc,grpcs",
		}
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Traefik
// ---------------------------------------------------------------------------

type traefikProvider struct{}

func (p *traefikProvider) IngressClass() string { return "traefik" }

func (p *traefikProvider) AppAnnotations() map[string]string {
	return map[string]string{
		"traefik.ingress.kubernetes.io/router.entrypoints": "websecure,web",
	}
}

func (p *traefikProvider) SystemAnnotations(serviceName string) map[string]string {
	ann := map[string]string{
		"traefik.ingress.kubernetes.io/router.entrypoints": "websecure,web",
	}
	return ann
}

func (p *traefikProvider) ProtocolAnnotations(protocol string) map[string]string {
	switch protocol {
	case "websocket":
		// Traefik supports WebSocket natively; just ensure correct entrypoints.
		return map[string]string{
			"traefik.ingress.kubernetes.io/router.entrypoints": "websecure,web",
		}
	case "grpc":
		return map[string]string{
			"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
			"traefik.ingress.kubernetes.io/service.serversscheme": "h2c",
		}
	default:
		return nil
	}
}
