package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/service"
)

func catalogCmd() *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:     "catalog",
		Short:   "List available backing services and plans",
		Aliases: []string{"marketplace", "m"},
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
				{"database", "Databases (Relational)"},
				{"nosql", "Databases (NoSQL / Document)"},
				{"datawarehouse", "Data Warehouses"},
				{"cache", "Caches"},
				{"messaging", "Message Queues"},
				{"streaming", "Stream / Event Processing"},
				{"storage", "Object Storage"},
				{"search", "Search / Analytics"},
				{"ai", "AI / ML Services"},
				{"media", "Media Services"},
				{"gateway", "API Gateways"},
			}

			total := 0
			for _, cat := range categories {
				printed := false
				for _, svc := range catalog {
					if svc.Category != cat.name {
						continue
					}
					if provider != "" && svc.Provider != provider {
						continue
					}
					if !printed {
						fmt.Printf("\n=== %s ===\n", cat.label)
						printed = true
					}
					providerBadge := ""
					switch svc.Provider {
					case "aws":
						providerBadge = " [AWS]"
					case "gcp":
						providerBadge = " [GCP]"
					case "azure":
						providerBadge = " [Azure]"
					}
					fmt.Printf("\n  %s%s  (%s)\n", svc.Name, providerBadge, svc.ID)
					fmt.Printf("    %s\n\n", svc.Description)
					fmt.Printf("    %-12s %-8s %-8s %-10s %s\n", "PLAN", "MEMORY", "CPU", "STORAGE", "COST")
					for _, p := range svc.Plans {
						stor := "—"
						if p.Resources.StorageGB > 0 {
							stor = fmt.Sprintf("%dGi", p.Resources.StorageGB)
						}
						mem := "—"
						if p.Resources.MemoryMB > 0 {
							mem = fmt.Sprintf("%dMi", p.Resources.MemoryMB)
						}
						cpu := "—"
						if p.Resources.CPUMillis > 0 {
							cpu = fmt.Sprintf("%dm", p.Resources.CPUMillis)
						}
						fmt.Printf("    %-12s %-8s %-8s %-10s %s\n",
							p.ID, mem, cpu, stor, p.CostEstimate)
					}
					total++
				}
			}
			if provider != "" {
				fmt.Printf("\n%d services (filtered by provider: %s)\n", total, provider)
			} else {
				fmt.Printf("\n%d services total\n", total)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Filter by provider (local, aws, gcp, azure)")

	return cmd
}
