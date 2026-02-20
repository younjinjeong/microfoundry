package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/service"
	"github.com/younjinjeong/microfoundry/pkg/terraform"
)

func topologyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Manage Terraform topology templates for service provisioning",
	}
	cmd.AddCommand(topologyUploadCmd(), topologyListCmd(), topologyDeleteCmd())
	return cmd
}

func topologyUploadCmd() *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "upload <service-type> <plan-id> <directory>",
		Short: "Upload Terraform files as a topology template",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceType, planID, dir := args[0], args[1], args[2]

			// Validate service type exists in catalog
			if _, ok := service.FindServiceType(serviceType); !ok {
				return fmt.Errorf("unknown service type %q — run 'mf catalog' to see available types", serviceType)
			}

			// Read .tf files from directory
			files := make(map[string]string)
			entries, err := os.ReadDir(dir)
			if err != nil {
				return fmt.Errorf("reading directory %s: %w", dir, err)
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
					continue
				}
				content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err != nil {
					return fmt.Errorf("reading %s: %w", entry.Name(), err)
				}
				files[entry.Name()] = string(content)
			}
			if len(files) == 0 {
				return fmt.Errorf("no .tf files found in %s", dir)
			}

			client, err := newK8sClient()
			if err != nil {
				return err
			}

			store := terraform.NewTopologyStore(client)
			ctx := context.Background()

			config := &models.TopologyConfig{
				ServiceType: serviceType,
				PlanID:      planID,
				Files:       files,
				Metadata: models.TopologyMetadata{
					Description: description,
				},
			}

			if err := store.Save(ctx, config); err != nil {
				return fmt.Errorf("saving topology: %w", err)
			}

			fmt.Printf("Topology uploaded: %s/%s (%d .tf files)\n", serviceType, planID, len(files))
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Topology description")
	return cmd
}

func topologyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all topology templates and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newK8sClient()
			if err != nil {
				return err
			}

			store := terraform.NewTopologyStore(client)
			ctx := context.Background()

			items, err := store.List(ctx, service.Catalog())
			if err != nil {
				return fmt.Errorf("listing topologies: %w", err)
			}

			fmt.Printf("%-30s %-10s %-10s %-5s %s\n", "SERVICE/PLAN", "STATUS", "VERSION", "FILES", "DESCRIPTION")
			for _, item := range items {
				status := "missing"
				ver := "-"
				files := "-"
				desc := ""
				if item.HasTopology {
					status = "ready"
					ver = fmt.Sprintf("v%d", item.Version)
					files = fmt.Sprintf("%d", item.FileCount)
					desc = item.Description
				}
				fmt.Printf("%-30s %-10s %-10s %-5s %s\n",
					item.ServiceType+"/"+item.PlanID, status, ver, files, desc)
			}
			return nil
		},
	}
}

func topologyDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <service-type> <plan-id>",
		Short: "Delete a topology template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newK8sClient()
			if err != nil {
				return err
			}

			store := terraform.NewTopologyStore(client)
			ctx := context.Background()

			if err := store.Delete(ctx, args[0], args[1]); err != nil {
				return fmt.Errorf("deleting topology: %w", err)
			}
			fmt.Printf("Topology deleted: %s/%s\n", args[0], args[1])
			return nil
		},
	}
}
