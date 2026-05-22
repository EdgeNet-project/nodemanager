package main

import (
	"fmt"
	"os"

	"edge-net.org/nodemanager/internal/config"
	"edge-net.org/nodemanager/internal/identity"
	"edge-net.org/nodemanager/internal/network"
	"edge-net.org/nodemanager/internal/onboarding"
	"edge-net.org/nodemanager/internal/preflight"
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
	preflightRes, err := preflight.Run(cmd.Context(), logger)
	if err != nil {
		logger.Fatal("Preflight checks failed", zap.Error(err))
	}
	logger.Info("Preflight checks completed",
		zap.String("public_ip", preflightRes.PublicIP),
		zap.Bool("nat_detected", preflightRes.NATDetected),
		zap.Bool("port_80_open", preflightRes.Port80Open),
		zap.Bool("port_443_open", preflightRes.Port443Open),
		zap.Bool("wg_supported", preflightRes.WGSupported),
	)

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

	// TODO: Initialize other components

	fmt.Println("Nodemanager exiting")
	os.Exit(0)
}
