package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/build"
	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/hosts"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/manifest"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/settings"
)

func pushCmd() *cobra.Command {
	var (
		memory    string
		instances int
		path      string
	)

	cmd := &cobra.Command{
		Use:   "push [app-name]",
		Short: "Deploy an application to MicroFoundry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Load config for active cluster
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			_, cluster, ok := cfg.Kubernetes.GetActiveCluster()
			if !ok {
				return fmt.Errorf("no active cluster configured")
			}
			domain := cluster.Domain

			// Determine source directory
			srcDir := path
			if srcDir == "" {
				srcDir, _ = os.Getwd()
			}
			srcDir, _ = filepath.Abs(srcDir)

			// Load manifest or build from flags
			var app models.App
			var routes []models.Route

			manifestPath := filepath.Join(srcDir, "manifest.yml")
			if _, err := os.Stat(manifestPath); err == nil {
				m, err := manifest.ReadFile(manifestPath)
				if err != nil {
					return fmt.Errorf("reading manifest: %w", err)
				}
				ma := m.Applications[0]
				if len(args) > 0 {
					ma.Name = args[0]
				}
				app = ma.ToApp(domain)
				routes = ma.ParseRoutes(domain)
			} else {
				if len(args) == 0 {
					return fmt.Errorf("app name required (no manifest.yml found)")
				}
				app = models.AppDefaults(args[0])
				routes = []models.Route{{Host: app.Name, Domain: domain}}
			}

			// Apply flag overrides
			if memory != "" {
				app.MemoryMB = manifest.ParseMemoryMB(memory)
			}
			if instances > 0 {
				app.Instances = instances
			}

			app.GUID = uuid.New().String()
			app.CreatedAt = time.Now()
			app.UpdatedAt = time.Now()

			// Set owner from current OS user
			if u, err := user.Current(); err == nil {
				if app.Env == nil {
					app.Env = make(map[string]string)
				}
				app.Env["MICROFOUNDRY_OWNER"] = u.Username
			}

			// Connect to K8s early so we can read registry settings
			k8sClient, err := k8s.NewClient(cluster.Context, cluster.Namespace, domain)
			if err != nil {
				return fmt.Errorf("connecting to kubernetes: %w", err)
			}

			// Determine image prefix from registry settings (if configured)
			imagePrefix := "microfoundry/"
			var registryCfg *models.RegistryConfig
			var registryPassword string
			store := settings.NewStore(k8sClient)
			if ps, err := store.Get(ctx); err == nil && ps.Registry.Enabled && ps.Registry.URL != "" {
				registryCfg = &ps.Registry
				imagePrefix = registryCfg.ImagePrefix()
				registryPassword, _ = store.GetCredential(ctx, "registry-password")
				fmt.Printf("Registry: %s (project: %s)\n", registryCfg.URL, registryCfg.Project)
			}

			// Phase 1: Build
			fmt.Printf("Building %s...\n", app.Name)
			strategy := build.DetectBuildStrategy(srcDir)
			fmt.Printf("  Strategy: %s\n", strategy)

			builder := build.NewBuilder(imagePrefix)
			result, err := builder.Build(app.Name, srcDir)
			if err != nil {
				fmt.Printf("  Build: FAILED\n")
				return fmt.Errorf("build failed: %w", err)
			}
			app.ImageRef = result.ImageRef
			app.State = models.AppStateDeploying
			fmt.Printf("  Build: OK (%s)\n", result.ImageRef)

			// Phase 1.5: Push to registry (if configured)
			if registryCfg != nil && registryPassword != "" {
				fmt.Printf("Pushing to registry %s...\n", registryCfg.URL)
				if err := builder.Login(registryCfg.URL, registryCfg.Username, registryPassword); err != nil {
					return fmt.Errorf("registry login failed: %w", err)
				}
				fmt.Printf("  Login: OK\n")

				if err := builder.Push(result.ImageRef); err != nil {
					return fmt.Errorf("registry push failed: %w", err)
				}
				fmt.Printf("  Push: OK (%s)\n", result.ImageRef)

				// Ensure imagePullSecret exists in namespace
				if err := k8sClient.EnsureImagePullSecret(ctx, registryCfg.URL, registryCfg.Username, registryPassword); err != nil {
					fmt.Printf("  ImagePullSecret: WARN (%v)\n", err)
				} else {
					fmt.Printf("  ImagePullSecret: OK\n")
				}
			}

			// Phase 2: Deploy to K8s
			fmt.Printf("Deploying %s to cluster %s...\n", app.Name, cfg.Kubernetes.Active)

			if err := k8sClient.EnsureNamespace(ctx); err != nil {
				return fmt.Errorf("namespace setup: %w", err)
			}

			if err := k8sClient.DeployApp(ctx, app, routes); err != nil {
				return fmt.Errorf("deploy failed: %w", err)
			}
			fmt.Printf("  Deployment: OK\n")

			// Phase 3: Create Ingress
			if len(routes) > 0 {
				if err := k8sClient.CreateIngress(ctx, app.Name, routes); err != nil {
					return fmt.Errorf("ingress creation failed: %w", err)
				}
				fmt.Printf("  Ingress: OK\n")
			}

			// Phase 4: Update hosts file
			for _, r := range routes {
				hostname := fmt.Sprintf("%s.%s", r.Host, r.Domain)
				if err := hosts.Add(hostname); err != nil {
					fmt.Printf("  Hosts: WARN (could not update hosts file: %v)\n", err)
					fmt.Printf("  TIP: Add '127.0.0.1 %s' to your hosts file manually\n", hostname)
				} else {
					fmt.Printf("  Hosts: OK (%s)\n", hostname)
				}
			}

			// Phase 5: Wait for rollout
			fmt.Printf("Waiting for app to start...\n")
			if err := k8sClient.WaitForRollout(ctx, app.Name, 120*time.Second); err != nil {
				fmt.Printf("  Warning: %v\n", err)
			}

			// Display result
			status, _ := k8sClient.GetAppStatus(ctx, app.Name)
			scheme := "http"
			if k8sClient.HasTLSSecret(ctx) {
				scheme = "https"
			}
			fmt.Println()
			fmt.Printf("name:       %s\n", app.Name)
			fmt.Printf("state:      STARTED\n")
			if len(routes) > 0 {
				fmt.Printf("routes:     %s://%s\n", scheme, routes[0].URL())
			}
			if status != nil {
				fmt.Printf("instances:  %d/%d\n", status.RunningCount, status.TotalCount)
			}
			fmt.Printf("memory:     %dM\n", app.MemoryMB)
			fmt.Printf("disk:       %dM\n", app.DiskMB)
			fmt.Printf("metrics:    auto-instrumented (Beyla eBPF)\n")
			fmt.Printf("dashboard:  %s/apps/%s?tab=performance\n", cfg.Admin.AdminURL(), app.Name)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&memory, "memory", "m", "", "Memory limit (e.g. 256M, 1G)")
	cmd.Flags().IntVarP(&instances, "instances", "i", 0, "Number of instances")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Path to app source directory")

	return cmd
}
