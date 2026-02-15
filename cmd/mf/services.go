package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func servicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "services",
		Short: "List provisioned service instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := service.NewManager(k8sClient)
			items, err := mgr.List(ctx)
			if err != nil {
				return err
			}

			if len(items) == 0 {
				fmt.Println("No service instances found.")
				fmt.Println("Use 'mf marketplace' to see available services.")
				return nil
			}

			fmt.Printf("%-20s %-30s %-12s %-12s %-5s\n", "NAME", "SERVICE", "PLAN", "STATUS", "APPS")
			for _, item := range items {
				fmt.Printf("%-20s %-30s %-12s %-12s %-5d\n",
					item.Name, item.ServiceType, item.Plan, item.Status, item.BoundApps)
			}
			return nil
		},
	}
}

func serviceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "service [name]",
		Short: "Show service instance details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			k8sClient, err := newK8sClient()
			if err != nil {
				return err
			}

			mgr := service.NewManager(k8sClient)
			inst, err := mgr.Get(ctx, name)
			if err != nil {
				return err
			}

			fmt.Printf("name:         %s\n", inst.Name)
			fmt.Printf("service:      %s\n", inst.ServiceType)
			fmt.Printf("plan:         %s\n", inst.Plan)
			fmt.Printf("status:       %s\n", inst.Status)
			if inst.StatusMsg != "" {
				fmt.Printf("message:      %s\n", inst.StatusMsg)
			}
			fmt.Printf("cluster:      %s\n", inst.ClusterID)

			if inst.Outputs.Host != "" {
				fmt.Println()
				fmt.Println("Connection Info:")
				fmt.Printf("  host:       %s\n", inst.Outputs.Host)
				fmt.Printf("  port:       %d\n", inst.Outputs.Port)
				fmt.Printf("  database:   %s\n", inst.Outputs.Database)
				fmt.Printf("  username:   %s\n", inst.Outputs.Username)
			}

			if len(inst.Bindings) > 0 {
				fmt.Println()
				fmt.Println("Bound Applications:")
				var apps []string
				for _, b := range inst.Bindings {
					apps = append(apps, b.AppName)
				}
				fmt.Printf("  %s\n", strings.Join(apps, ", "))
			}

			return nil
		},
	}
}
