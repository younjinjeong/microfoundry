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

			// Group by category
			categories := []struct {
				name  string
				label string
			}{
				{"database", "Databases"},
				{"cache", "Caches"},
				{"messaging", "Messaging"},
				{"storage", "Storage"},
				{"gateway", "API Gateways"},
			}

			for _, cat := range categories {
				printed := false
				for _, svc := range catalog {
					if svc.Category != cat.name {
						continue
					}
					if !printed {
						fmt.Printf("\n=== %s ===\n", cat.label)
						printed = true
					}
					fmt.Printf("\n  %s  (%s)\n", svc.Name, svc.ID)
					fmt.Printf("    %s\n\n", svc.Description)
					fmt.Printf("    %-12s %-8s %-8s %-10s %s\n", "PLAN", "MEMORY", "CPU", "STORAGE", "COST")
					for _, p := range svc.Plans {
						stor := "—"
						if p.Resources.StorageGB > 0 {
							stor = fmt.Sprintf("%dGi", p.Resources.StorageGB)
						}
						fmt.Printf("    %-12s %-8s %-8s %-10s %s\n",
							p.ID,
							fmt.Sprintf("%dMi", p.Resources.MemoryMB),
							fmt.Sprintf("%dm", p.Resources.CPUMillis),
							stor,
							p.CostEstimate)
					}
				}
			}
			fmt.Println()
			return nil
		},
	}
}
