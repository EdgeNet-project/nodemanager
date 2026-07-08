package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/EdgeNet-project/nodemanager/internal/config"
	"github.com/EdgeNet-project/nodemanager/internal/system/user"
	"go.uber.org/zap"
)

// User represents a user to be added to the system, as returned by the API.
type User struct {
	Username  string `json:"user"`
	PublicKey string `json:"public_key"`
	Sudo      bool   `json:"sudo"`
}

// Run performs the setup phase, which adds users and their public ssh keys from a REST API.
func Run(ctx context.Context, logger *zap.Logger, cfg *config.Config) error {
	logger.Info("Starting setup phase...")

	users, err := fetchUsers(ctx, cfg.Orchestrator.Host)
	if err != nil {
		logger.Error("Failed to fetch users from API", zap.Error(err))
		return err
	}

	for _, u := range users {
		logger.Info("Processing user", zap.String("user", u.Username))

		exists, err := user.Exists(u.Username)
		if err != nil {
			logger.Error("Failed to check if user exists", zap.String("user", u.Username), zap.Error(err))
			return err
		}

		if !exists {
			logger.Info("Adding user", zap.String("user", u.Username))
			if err := user.Add(u.Username); err != nil {
				logger.Error("Failed to add user", zap.String("user", u.Username), zap.Error(err))
				return err
			}

			if u.Sudo {
				logger.Info("Adding user to sudoers", zap.String("user", u.Username))
				if err := user.AddToSudoersNoPasswd(u.Username); err != nil {
					logger.Error("Failed to add user to sudoers", zap.String("user", u.Username), zap.Error(err))
					return err
				}
			}
		} else {
			logger.Info("User already exists, skipping user creation", zap.String("user", u.Username))
		}

		if u.PublicKey != "" {
			logger.Info("Adding SSH public key for user", zap.String("user", u.Username))
			if err := user.AddSSHKey(u.Username, u.PublicKey); err != nil {
				logger.Error("Failed to add SSH key", zap.String("user", u.Username), zap.Error(err))
				return err
			}
		} else {
			logger.Warn("No SSH public key provided for user", zap.String("user", u.Username))
		}
	}

	logger.Info("Setup phase completed successfully")
	return nil
}

func fetchUsers(ctx context.Context, host string) ([]User, error) {
	url := fmt.Sprintf("https://%s/install/setup", host)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}
