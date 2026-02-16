package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func bindServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bind-service <app-name> <service-instance>",
		Short: "Bind a service instance to an application",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			appName := args[0]
			svcName := args[1]

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := service.NewManager(k8sClient)
			binder := service.NewBinder(k8sClient, mgr)

			// Verify service exists and is available
			inst, err := mgr.Get(ctx, svcName)
			if err != nil {
				return err
			}
			if inst.Status != models.ServiceStatusAvailable {
				return fmt.Errorf("service %q is not available (status: %s)", svcName, inst.Status)
			}

			fmt.Printf("Binding service '%s' to app '%s'...\n", svcName, appName)

			// Add binding metadata
			if err := mgr.AddBinding(ctx, svcName, appName); err != nil {
				return fmt.Errorf("adding binding: %w", err)
			}

			// Inject credentials into deployment (envFrom + VCAP_SERVICES)
			if err := binder.Bind(ctx, appName, svcName); err != nil {
				// Rollback binding metadata
				_ = mgr.RemoveBinding(ctx, svcName, appName)
				return fmt.Errorf("binding to deployment: %w", err)
			}

			fmt.Printf("Service '%s' bound to app '%s'.\n", svcName, appName)
			fmt.Printf("The app will restart to pick up the new environment variables.\n")
			fmt.Printf("VCAP_SERVICES has been injected with credentials for '%s'.\n", svcName)
			return nil
		},
	}
}
