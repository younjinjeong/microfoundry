package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

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
			fmt.Printf("  Service: %s\n", serviceType)
			fmt.Printf("  Plan:    %s (%s)\n", plan, planInfo.InstanceClass)
			fmt.Printf("  Cost:    %s\n", planInfo.CostEstimate)

			if err := mgr.Create(ctx, inst); err != nil {
				return fmt.Errorf("creating service: %w", err)
			}

			// For now, immediately set to available with mock outputs
			// In production, this would trigger async Terraform apply
			password := generateCLIPassword(24)
			outputs := models.ServiceOutputs{
				Host:     fmt.Sprintf("%s.cluster.local", name),
				Port:     3306,
				Username: "admin",
				Password: password,
				Database: name,
				URI:      fmt.Sprintf("mysql://admin:%s@%s.cluster.local:3306/%s", password, name, name),
			}

			if err := mgr.SaveOutputs(ctx, name, outputs); err != nil {
				fmt.Printf("  Warning: could not save outputs: %v\n", err)
			}

			if err := mgr.UpdateStatus(ctx, name, models.ServiceStatusAvailable, ""); err != nil {
				fmt.Printf("  Warning: could not update status: %v\n", err)
			}

			fmt.Printf("\nService instance '%s' created successfully.\n", name)
			fmt.Printf("Use 'mf bind-service <app-name> %s' to bind it to an application.\n", name)
			return nil
		},
	}

	return cmd
}

func generateCLIPassword(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "fallback-change-me"
	}
	return hex.EncodeToString(b)[:length]
}
