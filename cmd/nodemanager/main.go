package main

import (
	"fmt"
	"os"

	"github.com/EdgeNet-project/nodemanager/internal/config"
	"github.com/EdgeNet-project/nodemanager/internal/heartbeat"
	"github.com/EdgeNet-project/nodemanager/internal/identity"
	"github.com/EdgeNet-project/nodemanager/internal/network"
	"github.com/EdgeNet-project/nodemanager/internal/onboarding"
	"github.com/EdgeNet-project/nodemanager/internal/preflight"
	"github.com/EdgeNet-project/nodemanager/internal/provisioner/kubernetes"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nodemanager",
		Short: "EdgeNet node manager",
		Run:   run,
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "/etc/edgenet/nodemanager.conf", "path to config file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Info("Starting EdgeNet node manager",
		zap.String("server", cfg.Server),
	)

	/**
	 *	1. Load or create Node identity
	 *	Node identity is a Code generated the first time the nodemanager is started.
	 *	This Code is used to identify the Node in the EdgeNet cluster and is used for authentication and authorization.
	 *  The identity persists across nodemanager restarts and reboots.
	 */
	id, err := identity.LoadOrCreate(cfg.Identity)
	if err != nil {
		logger.Fatal("Failed to load or create identity", zap.Error(err))
	}

	logger.Info("Node identity loaded",
		zap.String("code", id.Code),
	)

	/**
	 * 2. Preflight checks
	 * Checks network connectivity and system configuration.
	 */
	logger.Info("Running preflight checks...")
	preflightRes, err := preflight.Run(cmd.Context(), logger, cfg.Orchestrator.Host)
	if err != nil {
		logger.Fatal("Preflight checks failed", zap.Error(err))
	}

	/**
	 * 3. Onboarding
	 * Performs checkin with the server, change hostname
	 * and waits until the node is ENABLED.
	 */
	logger.Info("Starting onboarding process...")
	if err := onboarding.Run(cmd.Context(), logger, cfg, id); err != nil {
		logger.Fatal("Onboarding failed", zap.Error(err))
	}

	logger.Info("Onboarding completed successfully")

	/**
	 * 4. Networking: wiregard configuration
	 */
	logger.Info("Starting WireGuard configuration...")
	if err := network.SetupWireguard(cmd.Context(), logger, cfg, id); err != nil {
		logger.Fatal("WireGuard setup failed", zap.Error(err))
	}
	logger.Info("WireGuard setup completed successfully")

	/**
	 * 5. Provisioning: kubernetes configuration
	 */
	logger.Info("Starting Kubernetes provisioning phase...")
	prov := kubernetes.New(logger, cfg)

	isProv, err := prov.IsProvisioned(cmd.Context())
	if err == nil && isProv {
		logger.Info("Kubernetes is already provisioned, skipping.")
	} else {
		if err := prov.Provision(cmd.Context(), *id); err != nil {
			logger.Error("Kubernetes provisioning failed", zap.Error(err))
			// We don't necessarily want to exit here if we want the agent to keep running
			// but for now, let's follow the existing pattern of fatal errors for setup.
			os.Exit(1)
		}
		logger.Info("Kubernetes provisioning completed successfully")
	}

	/**
	 * 6. Heartbeat: ping the orchestrator
	 */
	logger.Info("Starting heartbeat...")
	go heartbeat.Run(cmd.Context(), logger, cfg)

	fmt.Println("Nodemanager running, press Ctrl+C to exit")
	<-cmd.Context().Done()
}
