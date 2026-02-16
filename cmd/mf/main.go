package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
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
		adminCmd(),
		catalogCmd(),
		createServiceCmd(),
		servicesCmd(),
		serviceCmd(),
		bindServiceCmd(),
		unbindServiceCmd(),
		deleteServiceCmd(),
		secretsListCmd(),
		secretDetailCmd(),
		createSecretCmd(),
		deleteSecretCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newK8sClient loads config and returns a k8s.Client for the active cluster.
func newK8sClient() (*k8s.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	_, cluster, ok := cfg.Kubernetes.GetActiveCluster()
	if !ok {
		return nil, fmt.Errorf("no active cluster configured")
	}
	return k8s.NewClient(cluster.Context, cluster.Namespace, cluster.Domain)
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
