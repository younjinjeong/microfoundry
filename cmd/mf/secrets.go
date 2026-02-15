package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/secrets"
)

func secretsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secrets",
		Short: "List all managed secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := secrets.NewManager(k8sClient)
			items, err := mgr.List(ctx)
			if err != nil {
				return err
			}

			if len(items) == 0 {
				fmt.Println("No managed secrets found.")
				fmt.Println("Use 'mf create-secret <name> key=value...' to create a secret.")
				return nil
			}

			fmt.Printf("%-25s %-15s %-20s %-6s %-10s\n", "NAME", "TYPE", "SERVICE", "KEYS", "CREATED")
			for _, item := range items {
				svc := "—"
				if item.ServiceInstance != "" {
					svc = item.ServiceInstance
				}
				fmt.Printf("%-25s %-15s %-20s %-6d %-10s\n",
					item.K8sName, item.Type, svc, item.KeyCount, item.CreatedAt.Format("2006-01-02"))
			}
			return nil
		},
	}
}

func secretDetailCmd() *cobra.Command {
	var reveal bool

	cmd := &cobra.Command{
		Use:   "secret <name>",
		Short: "Show secret details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := secrets.NewManager(k8sClient)
			detail, err := mgr.Get(ctx, name)
			if err != nil {
				return err
			}

			fmt.Printf("name:         %s\n", detail.Name)
			fmt.Printf("k8s-name:     %s\n", detail.K8sName)
			fmt.Printf("type:         %s\n", detail.Type)
			if detail.ServiceInstance != "" {
				fmt.Printf("service:      %s\n", detail.ServiceInstance)
			}
			if detail.Description != "" {
				fmt.Printf("description:  %s\n", detail.Description)
			}
			fmt.Printf("keys:         %s\n", strings.Join(detail.Keys, ", "))
			fmt.Printf("created:      %s\n", detail.CreatedAt.Format("2006-01-02T15:04:05Z"))

			if len(detail.Keys) > 0 {
				fmt.Println()
				fmt.Println("Key-Value Pairs:")
				for _, key := range detail.Keys {
					if reveal {
						val, err := mgr.GetValue(ctx, name, key)
						if err != nil {
							fmt.Printf("  %-15s %s\n", key+":", "(error: "+err.Error()+")")
						} else {
							fmt.Printf("  %-15s %s\n", key+":", val)
						}
					} else {
						fmt.Printf("  %-15s %s\n", key+":", "********")
					}
				}
			}

			if len(detail.BoundApps) > 0 {
				fmt.Println()
				fmt.Println("Bound Applications:")
				for _, app := range detail.BoundApps {
					fmt.Printf("  %s\n", app)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&reveal, "reveal", false, "Show actual secret values")
	return cmd
}

func createSecretCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-secret <name> [key=value...]",
		Short: "Create a user-defined secret",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			if !models.ValidSecretName.MatchString(name) {
				return fmt.Errorf("invalid secret name %q: must be lowercase alphanumeric with hyphens, 2-42 characters", name)
			}

			// Parse key=value pairs
			data := make(map[string]string)
			for _, kv := range args[1:] {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid key=value pair: %q (expected KEY=VALUE format)", kv)
				}
				if parts[0] == "" {
					return fmt.Errorf("empty key in pair: %q", kv)
				}
				data[parts[0]] = parts[1]
			}

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := secrets.NewManager(k8sClient)

			fmt.Printf("Creating secret '%s' with %d key(s)...\n", name, len(data))
			if err := mgr.CreateUserSecret(ctx, name, "", data); err != nil {
				return fmt.Errorf("creating secret: %w", err)
			}

			fmt.Printf("Secret '%s' created successfully.\n", name)
			fmt.Printf("K8s Secret name: %s%s\n", models.UserSecretPrefix, name)
			return nil
		},
	}
}

func deleteSecretCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-secret <name>",
		Short: "Delete a user-defined secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := secrets.NewManager(k8sClient)

			// Check if it's a service secret
			detail, err := mgr.Get(ctx, name)
			if err != nil {
				return err
			}

			if detail.Type == models.SecretTypeService {
				return fmt.Errorf("cannot delete service secret %q — use 'mf delete-service %s' instead", name, detail.ServiceInstance)
			}

			if len(detail.BoundApps) > 0 {
				return fmt.Errorf("secret %q has %d active binding(s) — unbind all apps first", name, len(detail.BoundApps))
			}

			fmt.Printf("Deleting secret '%s'...\n", name)
			if err := mgr.Delete(ctx, name); err != nil {
				return fmt.Errorf("deleting secret: %w", err)
			}

			fmt.Printf("Secret '%s' deleted.\n", name)
			return nil
		},
	}
}
