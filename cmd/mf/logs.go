package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
)

func logsCmd() *cobra.Command {
	var recent bool

	cmd := &cobra.Command{
		Use:   "logs [app-name]",
		Short: "Stream or fetch application logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			k8sClient, err := k8s.NewClient("docker-desktop", "microfoundry", "cf-local.dev")
			if err != nil {
				return err
			}

			follow := !recent
			stream, err := k8sClient.GetAppLogs(ctx, name, follow)
			if err != nil {
				return fmt.Errorf("getting logs for %s: %w", name, err)
			}
			defer stream.Close()

			_, err = io.Copy(os.Stdout, stream)
			if err != nil && err != io.EOF {
				return err
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&recent, "recent", false, "Show recent logs instead of streaming")
	return cmd
}
