package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/admin"
	"github.com/younjinjeong/microfoundry/pkg/auth"
	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/csp"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/monitoring"
)

func adminCmd() *cobra.Command {
	var port int
	var host string
	var tlsCert string
	var tlsKey string

	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Start the MicroFoundry admin web interface",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			clientManager := k8s.NewClientManager(
				cfg.Kubernetes.Clusters,
				cfg.Kubernetes.Active,
			)

			// Build server options
			var opts []admin.ServerOption

			// Initialize auth if enabled
			if cfg.Auth.Enabled {
				ctx := context.Background()
				sessions := auth.NewSessionManager(cfg.Auth.SessionKey)

				// Get active K8s client for org store
				activeClient, err := clientManager.GetActiveClient()
				if err != nil {
					return fmt.Errorf("connecting to cluster for auth: %w", err)
				}

				orgStore := auth.NewOrgStore(activeClient.Clientset, activeClient.Namespace)
				wsStore := auth.NewWorkspaceStore(activeClient.Clientset, activeClient.Namespace)

				authCfg := auth.AuthConfig{
					Enabled:      cfg.Auth.Enabled,
					IssuerURL:    cfg.Auth.IssuerURL,
					ClientID:     cfg.Auth.ClientID,
					ClientSecret: cfg.Auth.ClientSecret,
					RedirectURL:  cfg.Auth.RedirectURL,
					SessionKey:   cfg.Auth.SessionKey,
				}

				oidcAuth, err := auth.NewOIDCAuthenticator(ctx, authCfg, sessions, orgStore, wsStore)
				if err != nil {
					return fmt.Errorf("initializing OIDC: %w", err)
				}

				opts = append(opts, admin.WithAuth(oidcAuth, sessions, orgStore))
				opts = append(opts, admin.WithWorkspaceStore(wsStore))
				fmt.Printf("Authentication enabled (Keycloak: %s)\n", cfg.Auth.IssuerURL)

				// Keycloak Admin API client (optional — needs admin_base_url)
				if cfg.Auth.AdminBaseURL != "" {
					kcAdmin := auth.NewKeycloakAdminClient(
						cfg.Auth.AdminBaseURL,
						cfg.Auth.Realm,
						cfg.Auth.AdminClientID,
						cfg.Auth.AdminClientSecret,
					)
					opts = append(opts, admin.WithKeycloakAdmin(kcAdmin))
					fmt.Printf("Keycloak Admin API enabled (%s)\n", cfg.Auth.AdminBaseURL)
				}

				// OPA authorization engine
				opaEngine, err := auth.NewOPAEngine()
				if err != nil {
					return fmt.Errorf("initializing OPA engine: %w", err)
				}
				opts = append(opts, admin.WithOPA(opaEngine))
				fmt.Println("OPA authorization enabled")

				// Audit log
				auditLog := auth.NewAuditLog(1000)
				opts = append(opts, admin.WithAuditLog(auditLog))
			}

			// CSP credential manager (OIDC federation via Keycloak)
			if cfg.Auth.Enabled && cfg.Auth.IssuerURL != "" {
				tokenURL := cfg.Auth.IssuerURL + "/protocol/openid-connect/token"
				clientID := cfg.Auth.AdminClientID
				clientSecret := cfg.Auth.AdminClientSecret
				if clientID == "" {
					clientID = cfg.Auth.ClientID
				}
				if clientSecret == "" {
					clientSecret = cfg.Auth.ClientSecret
				}
				cspMgr := csp.NewManager(tokenURL, clientID, clientSecret)
				opts = append(opts, admin.WithCSPManager(cspMgr))
				fmt.Println("CSP OIDC federation enabled")
			}

			// Resolve TLS cert/key paths
			// Priority: CLI flags → config file paths → auto-detect from ~/.mf/
			certPath, keyPath := tlsCert, tlsKey
			if certPath == "" && keyPath == "" && cfg.Admin.TLSCert != "" && cfg.Admin.TLSKey != "" {
				certPath = cfg.Admin.TLSCert
				keyPath = cfg.Admin.TLSKey
			}
			if certPath == "" && keyPath == "" && cfg.Admin.TLS {
				// Auto-detect from ~/.mf/ directory
				if home, err := os.UserHomeDir(); err == nil {
					cp := filepath.Join(home, ".mf", "cert.pem")
					kp := filepath.Join(home, ".mf", "key.pem")
					if _, err := os.Stat(cp); err == nil {
						if _, err := os.Stat(kp); err == nil {
							certPath = cp
							keyPath = kp
						}
					}
				}
			}

			tlsEnabled := certPath != "" && keyPath != ""

			if cfg.Admin.TLS && !tlsEnabled {
				fmt.Println("Warning: admin.tls is true but certificates not found at ~/.mf/cert.pem")
				fmt.Println("  Falling back to HTTP. Run 'mf setup tls' to generate certificates.")
				fmt.Println()
			}

			if tlsEnabled {
				opts = append(opts, admin.WithTLS(certPath, keyPath))
			}

			// Resolve listen port
			// CLI flag overrides config; otherwise use tls_port for HTTPS, port for HTTP
			listenPort := port
			if !cmd.Flags().Changed("port") {
				if tlsEnabled && cfg.Admin.TLSPort > 0 {
					listenPort = cfg.Admin.TLSPort
				} else if cfg.Admin.Port > 0 {
					listenPort = cfg.Admin.Port
				}
			}

			// Resolve listen host
			listenHost := host
			if !cmd.Flags().Changed("host") && cfg.Admin.Host != "" {
				listenHost = cfg.Admin.Host
			}

			// Pass admin domain to server for template rendering
			if cfg.Admin.Domain != "" {
				opts = append(opts, admin.WithDomain(cfg.Admin.Domain, tlsEnabled))
			}

			srv := admin.NewServer(clientManager, cfg, version, opts...)

			// Start background metrics collector
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go monitoring.StartCollector(ctx, srv.GetMetrics(), clientManager, 30*time.Second)

			// Startup banner
			fmt.Println()
			fmt.Println("MicroFoundry Admin")
			fmt.Println()
			if tlsEnabled && cfg.Admin.Domain != "" {
				adminURL := cfg.Admin.AdminURL()
				fmt.Printf("  HTTPS  %s\n", adminURL)
				fmt.Printf("  Cluster  %s\n", cfg.Kubernetes.Active)
				fmt.Printf("  Domain   %s\n", cfg.Admin.Domain)
				fmt.Printf("  TLS      %s (trusted by mkcert)\n", certPath)
			} else if tlsEnabled {
				fmt.Printf("  HTTPS  https://localhost:%d\n", listenPort)
				fmt.Printf("  Cluster  %s\n", cfg.Kubernetes.Active)
				fmt.Printf("  TLS      %s\n", certPath)
			} else {
				fmt.Printf("  HTTP   http://localhost:%d\n", listenPort)
				fmt.Printf("  Cluster  %s\n", cfg.Kubernetes.Active)
				if cfg.Admin.Domain == "" {
					fmt.Println()
					fmt.Println("  TIP: Run 'mf setup tls' to enable HTTPS with a custom domain")
				}
			}
			fmt.Println()

			// Start HTTP redirect server when TLS is enabled
			if tlsEnabled {
				httpPort := cfg.Admin.Port
				if httpPort == 0 {
					httpPort = 8080
				}
				redirectURL := cfg.Admin.AdminURL()
				go func() {
					redirectMux := http.NewServeMux()
					redirectMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
						target := redirectURL + r.URL.Path
						if r.URL.RawQuery != "" {
							target += "?" + r.URL.RawQuery
						}
						http.Redirect(w, r, target, http.StatusMovedPermanently)
					})
					httpAddr := fmt.Sprintf("%s:%d", listenHost, httpPort)
					_ = http.ListenAndServe(httpAddr, redirectMux)
				}()
			}

			addr := fmt.Sprintf("%s:%d", listenHost, listenPort)
			return srv.ListenAndServe(addr)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind to")
	cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "Path to TLS certificate file (enables HTTPS)")
	cmd.Flags().StringVar(&tlsKey, "tls-key", "", "Path to TLS private key file (enables HTTPS)")

	return cmd
}
