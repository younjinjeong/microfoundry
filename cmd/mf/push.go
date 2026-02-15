package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/build"
	"github.com/younjinjeong/microfoundry/pkg/hosts"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/manifest"
	"github.com/younjinjeong/microfoundry/pkg/models"
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

			// Determine source directory
			srcDir := path
			if srcDir == "" {
				srcDir, _ = os.Getwd()
			}
			srcDir, _ = filepath.Abs(srcDir)

			// Load manifest or build from flags
			var app models.App
			var routes []models.Route
			domain := "cf-local.dev"

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

			// Phase 1: Build
			fmt.Printf("Building %s...\n", app.Name)
			strategy := build.DetectBuildStrategy(srcDir)
			fmt.Printf("  Strategy: %s\n", strategy)

			builder := build.NewBuilder("microfoundry/")
			result, err := builder.Build(app.Name, srcDir)
			if err != nil {
				fmt.Printf("  Build: FAILED\n")
				return fmt.Errorf("build failed: %w", err)
			}
			app.ImageRef = result.ImageRef
			app.State = models.AppStateDeploying
			fmt.Printf("  Build: OK (%s)\n", result.ImageRef)

			// Phase 2: Deploy to K8s
			fmt.Printf("Deploying %s...\n", app.Name)
			k8sClient, err := k8s.NewClient("docker-desktop", "microfoundry", domain)
			if err != nil {
				return fmt.Errorf("connecting to kubernetes: %w", err)
			}

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
			fmt.Println()
			fmt.Printf("name:       %s\n", app.Name)
			fmt.Printf("state:      STARTED\n")
			if len(routes) > 0 {
				fmt.Printf("routes:     %s\n", routes[0].URL())
			}
			if status != nil {
				fmt.Printf("instances:  %d/%d\n", status.RunningCount, status.TotalCount)
			}
			fmt.Printf("memory:     %dM\n", app.MemoryMB)
			fmt.Printf("disk:       %dM\n", app.DiskMB)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&memory, "memory", "m", "", "Memory limit (e.g. 256M, 1G)")
	cmd.Flags().IntVarP(&instances, "instances", "i", 0, "Number of instances")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Path to app source directory")

	return cmd
}
