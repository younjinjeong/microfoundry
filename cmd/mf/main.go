package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "mf",
		Short:   "MicroFoundry — micro CloudFoundry for Kubernetes",
		Version: version,
	}

	rootCmd.AddCommand(
		versionCmd(),
		pushCmd(),
		appsCmd(),
		appCmd(),
		logsCmd(),
		deleteCmd(),
		scaleCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the MicroFoundry version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("MicroFoundry %s\n", version)
		},
	}
}
