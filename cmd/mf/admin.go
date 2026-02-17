package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/admin"
	"github.com/younjinjeong/microfoundry/pkg/auth"
	"github.com/younjinjeong/microfoundry/pkg/config"
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

				authCfg := auth.AuthConfig{
					Enabled:      cfg.Auth.Enabled,
					IssuerURL:    cfg.Auth.IssuerURL,
					ClientID:     cfg.Auth.ClientID,
					ClientSecret: cfg.Auth.ClientSecret,
					RedirectURL:  cfg.Auth.RedirectURL,
					SessionKey:   cfg.Auth.SessionKey,
				}

				oidcAuth, err := auth.NewOIDCAuthenticator(ctx, authCfg, sessions, orgStore)
				if err != nil {
					return fmt.Errorf("initializing OIDC: %w", err)
				}

				opts = append(opts, admin.WithAuth(oidcAuth, sessions, orgStore))
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

			// TLS support
			if tlsCert != "" && tlsKey != "" {
				opts = append(opts, admin.WithTLS(tlsCert, tlsKey))
			}

			srv := admin.NewServer(clientManager, cfg, version, opts...)

			// Start background metrics collector
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go monitoring.StartCollector(ctx, srv.GetMetrics(), clientManager, 30*time.Second)

			addr := fmt.Sprintf("%s:%d", host, port)
			scheme := "http"
			if tlsCert != "" && tlsKey != "" {
				scheme = "https"
			}
			fmt.Printf("MicroFoundry Admin starting at %s://localhost:%d\n", scheme, port)
			fmt.Printf("Active cluster: %s\n", cfg.Kubernetes.Active)
			return srv.ListenAndServe(addr)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind to")
	cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "Path to TLS certificate file (enables HTTPS)")
	cmd.Flags().StringVar(&tlsKey, "tls-key", "", "Path to TLS private key file (enables HTTPS)")

	return cmd
}
