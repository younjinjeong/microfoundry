package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/hosts"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
)

func deleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [app-name]",
		Short: "Delete a deployed application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]
			domain := "cf-local.dev"

			k8sClient, err := k8s.NewClient("docker-desktop", "microfoundry", domain)
			if err != nil {
				return err
			}

			// Get routes before deleting (for hosts file cleanup)
			ingressMap, _ := k8sClient.ListIngresses(ctx)

			fmt.Printf("Deleting %s...\n", name)

			if err := k8sClient.DeleteApp(ctx, name); err != nil {
				return fmt.Errorf("deleting %s: %w", name, err)
			}

			// Clean up hosts entries
			if hostnames, ok := ingressMap[name]; ok {
				for _, h := range hostnames {
					_ = hosts.Remove(h)
				}
			}

			fmt.Printf("App %s deleted.\n", name)
			return nil
		},
	}
}
