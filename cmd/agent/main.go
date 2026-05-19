package main

import (
	"fmt"
	"os"

	"github.com/edgenet-project/edgenet-agent/internal/config"
	"github.com/edgenet-project/edgenet-agent/internal/identity"
	"github.com/edgenet-project/edgenet-agent/internal/network"
	"github.com/edgenet-project/edgenet-agent/internal/onboarding"
	"github.com/edgenet-project/edgenet-agent/internal/preflight"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "edgenet-agent",
		Short: "EdgeNet Node Agent",
		Run:   run,
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "/etc/edgenet/agent.conf", "path to config file")

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

	logger.Info("Starting EdgeNet Node Agent",
		zap.String("server", cfg.Server),
	)

	/**
	 *	1. Load or create Node identity
	 *	Node identity is a Code generated the first time the agent is started.
	 *	This Code is used to identify the Node in the EdgeNet cluster and is used for authentication and authorization.
	 *  The identity persists across agent restarts and reboots.
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

	fmt.Println("EdgeNet Node Agent Skeleton")
	os.Exit(0)
}
