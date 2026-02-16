package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func createServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-service <service-type> <plan> <instance-name>",
		Short: "Provision a new backing service instance",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			serviceType := args[0]
			plan := args[1]
			name := args[2]

			// Validate instance name
			if !models.ValidServiceName.MatchString(name) {
				return fmt.Errorf("invalid service name %q: must be lowercase alphanumeric with hyphens, 2-42 characters", name)
			}

			// Validate service type and plan exist
			_, ok := service.FindServiceType(serviceType)
			if !ok {
				return fmt.Errorf("unknown service type %q — run 'mf marketplace' to see available services", serviceType)
			}
			planInfo, ok := service.FindPlan(serviceType, plan)
			if !ok {
				return fmt.Errorf("unknown plan %q for service %q", plan, serviceType)
			}

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := service.NewManager(k8sClient)

			// Check if already exists
			if existing, _ := mgr.Get(ctx, name); existing != nil {
				return fmt.Errorf("service instance %q already exists (status: %s)", name, existing.Status)
			}

			inst := &models.ServiceInstance{
				Name:        name,
				ServiceType: serviceType,
				Plan:        plan,
				ClusterID:   "active",
			}

			fmt.Printf("Creating service instance '%s'...\n", name)
			fmt.Printf("  Service:  %s\n", serviceType)
			fmt.Printf("  Plan:     %s\n", planInfo.Name)
			fmt.Printf("  Memory:   %dMi\n", planInfo.Resources.MemoryMB)
			fmt.Printf("  CPU:      %dm\n", planInfo.Resources.CPUMillis)
			if planInfo.Resources.StorageGB > 0 {
				fmt.Printf("  Storage:  %dGi\n", planInfo.Resources.StorageGB)
			}
			fmt.Printf("  Cost:     %s\n", planInfo.CostEstimate)

			if err := mgr.Create(ctx, inst); err != nil {
				return fmt.Errorf("creating service: %w", err)
			}

			// Wait for provisioning to complete
			fmt.Printf("\nProvisioning")
			result, err := mgr.WaitForStatus(ctx, name, models.ServiceStatusAvailable, 120*time.Second)
			fmt.Println()

			if err != nil {
				return fmt.Errorf("provisioning service: %w", err)
			}

			if result.Status == models.ServiceStatusFailed {
				return fmt.Errorf("provisioning failed: %s", result.StatusMsg)
			}

			fmt.Printf("\nService instance '%s' created successfully.\n", name)
			if result.Outputs.Host != "" {
				fmt.Printf("  Host: %s\n", result.Outputs.Host)
			}
			if result.Outputs.Port > 0 {
				fmt.Printf("  Port: %d\n", result.Outputs.Port)
			}
			fmt.Printf("\nUse 'mf bind-service <app-name> %s' to bind it to an application.\n", name)
			return nil
		},
	}

	return cmd
}
