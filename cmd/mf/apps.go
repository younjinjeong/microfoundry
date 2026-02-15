package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
)

func appsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apps",
		Short: "List all deployed applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			k8sClient, err := k8s.NewClient("docker-desktop", "microfoundry", "cf-local.dev")
			if err != nil {
				return err
			}

			names, err := k8sClient.ListApps(ctx)
			if err != nil {
				return err
			}

			if len(names) == 0 {
				fmt.Println("No apps found.")
				return nil
			}

			// Get ingress routes for display
			ingressMap, _ := k8sClient.ListIngresses(ctx)

			fmt.Printf("%-20s %-10s %-12s %s\n", "name", "state", "instances", "routes")
			for _, name := range names {
				status, err := k8sClient.GetAppStatus(ctx, name)
				state := "unknown"
				instanceStr := "?"
				if err == nil && status != nil {
					if status.RunningCount > 0 {
						state = "started"
					} else {
						state = "stopped"
					}
					instanceStr = fmt.Sprintf("%d/%d", status.RunningCount, status.TotalCount)
				}

				routeStr := ""
				if hosts, ok := ingressMap[name]; ok {
					routeStr = strings.Join(hosts, ", ")
				}

				fmt.Printf("%-20s %-10s %-12s %s\n", name, state, instanceStr, routeStr)
			}

			return nil
		},
	}
}

func appCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "app [name]",
		Short: "Show application details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			k8sClient, err := k8s.NewClient("docker-desktop", "microfoundry", "cf-local.dev")
			if err != nil {
				return err
			}

			status, err := k8sClient.GetAppStatus(ctx, name)
			if err != nil {
				return fmt.Errorf("app %s not found: %w", name, err)
			}

			ingressMap, _ := k8sClient.ListIngresses(ctx)

			state := "stopped"
			if status.RunningCount > 0 {
				state = "started"
			}

			fmt.Printf("name:       %s\n", name)
			fmt.Printf("state:      %s\n", state)
			fmt.Printf("instances:  %d/%d\n", status.RunningCount, status.TotalCount)
			if hosts, ok := ingressMap[name]; ok {
				fmt.Printf("routes:     %s\n", strings.Join(hosts, ", "))
			}
			fmt.Println()

			if len(status.Instances) > 0 {
				fmt.Printf("     %-10s %-25s\n", "state", "since")
				for _, inst := range status.Instances {
					since := ""
					if !inst.Since.IsZero() {
						since = inst.Since.Format("2006-01-02T15:04:05Z")
					}
					fmt.Printf("#%-3d %-10s %-25s\n", inst.Index, strings.ToLower(inst.State), since)
				}
			}

			return nil
		},
	}
}
