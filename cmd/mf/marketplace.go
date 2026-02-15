package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func marketplaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "marketplace",
		Short:   "List available backing services and plans",
		Aliases: []string{"m"},
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := service.Catalog()

			if len(catalog) == 0 {
				fmt.Println("No services available.")
				return nil
			}

			for _, svc := range catalog {
				fmt.Printf("\n%s  (%s)\n", svc.Name, svc.ID)
				fmt.Printf("  %s\n", svc.Description)
				fmt.Printf("  Provider: %s  |  Category: %s\n", svc.Provider, svc.Category)
				fmt.Println()
				fmt.Printf("  %-15s %-20s %-10s %-10s %-8s %s\n", "PLAN", "INSTANCE", "STORAGE", "REPLICAS", "AZ", "COST")
				for _, p := range svc.Plans {
					replicas := "none"
					if p.Replicas > 0 {
						replicas = fmt.Sprintf("%d", p.Replicas)
					}
					az := "single"
					if p.MultiAZ {
						az = "multi"
					}
					fmt.Printf("  %-15s %-20s %-10s %-10s %-8s %s\n",
						p.ID, p.InstanceClass, fmt.Sprintf("%dGB", p.StorageGB),
						replicas, az, p.CostEstimate)
				}
			}
			fmt.Println()
			return nil
		},
	}
}
