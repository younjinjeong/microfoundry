package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func unbindServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unbind-service <app-name> <service-instance>",
		Short: "Unbind a service instance from an application",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			appName := args[0]
			svcName := args[1]

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := service.NewManager(k8sClient, nil)
			binder := service.NewBinder(k8sClient, mgr)

			fmt.Printf("Unbinding service '%s' from app '%s'...\n", svcName, appName)

			// Remove credentials from deployment (envFrom + VCAP_SERVICES)
			if err := binder.Unbind(ctx, appName, svcName); err != nil {
				return fmt.Errorf("unbinding from deployment: %w", err)
			}

			// Remove binding metadata
			if err := mgr.RemoveBinding(ctx, svcName, appName); err != nil {
				return fmt.Errorf("removing binding: %w", err)
			}

			fmt.Printf("Service '%s' unbound from app '%s'.\n", svcName, appName)
			return nil
		},
	}
}
