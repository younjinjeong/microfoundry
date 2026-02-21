package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func deleteServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-service <instance-name>",
		Short: "Delete a provisioned service instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := service.NewManager(k8sClient, nil)

			// Verify exists
			inst, err := mgr.Get(ctx, name)
			if err != nil {
				return err
			}

			// Check for active bindings
			if len(inst.Bindings) > 0 {
				return fmt.Errorf("service %q has %d active binding(s) — unbind all apps first", name, len(inst.Bindings))
			}

			fmt.Printf("Deleting service instance '%s' (deprovisioning K8s resources)...\n", name)

			if err := mgr.Delete(ctx, name); err != nil {
				return fmt.Errorf("deleting service: %w", err)
			}

			fmt.Printf("Service instance '%s' deleted.\n", name)
			return nil
		},
	}
}
