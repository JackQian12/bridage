package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nuts/bridage/internal/config"
	"github.com/nuts/bridage/internal/logger"
	"github.com/nuts/bridage/internal/models"
	"github.com/nuts/bridage/internal/security"
	"github.com/nuts/bridage/internal/store/postgres"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var rootCmd = &cobra.Command{
	Use:   "bridagectl",
	Short: "bridage management CLI",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(keysCmd())
	rootCmd.AddCommand(adminCmd())
	rootCmd.AddCommand(providersCmd())
}

// ─── shared pool helper ───────────────────────────────────────────────────────

func mustPool() (*postgres.APIKeyStore, *postgres.AdminStore, *postgres.ProviderStore, func()) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	log, _ := logger.New("warn")
	_ = log
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db error:", err)
		os.Exit(1)
	}
	return postgres.NewAPIKeyStore(pool),
		postgres.NewAdminStore(pool),
		postgres.NewProviderStore(pool),
		func() { pool.Close() }
}

// ─── keys ─────────────────────────────────────────────────────────────────────

func keysCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "keys", Short: "Manage API keys"}

	// keys list
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all API keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			apiKeys, _, _, cleanup := mustPool()
			defer cleanup()
			list, err := apiKeys.List(context.Background())
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(list)
		},
	})

	// keys create
	var keyName string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new downstream API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			apiKeys, _, _, cleanup := mustPool()
			defer cleanup()

			plaintext, hash, err := security.GenerateAPIKey()
			if err != nil {
				return err
			}
			index := security.HashAPIKeyFast(plaintext)

			k, err := apiKeys.Create(context.Background(), &models.APIKey{
				Name:     keyName,
				KeyHash:  hash,
				KeyIndex: index,
				Status:   models.APIKeyStatusActive,
			})
			if err != nil {
				return err
			}
			fmt.Printf("API Key created (id: %s).\nKey (save this, it won't be shown again):\n%s\n", k.ID, plaintext)
			return nil
		},
	}
	createCmd.Flags().StringVarP(&keyName, "name", "n", "", "Name for the key (required)")
	_ = createCmd.MarkFlagRequired("name")
	cmd.AddCommand(createCmd)

	return cmd
}

// ─── admin ────────────────────────────────────────────────────────────────────

func adminCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "admin", Short: "Manage admin users"}

	var username, password string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an admin user",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, adminStore, _, cleanup := mustPool()
			defer cleanup()
			hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			user, err := adminStore.Create(context.Background(), username, string(hashed))
			if err != nil {
				return err
			}
			fmt.Printf("Admin user '%s' created (id: %s)\n", user.Username, user.ID)
			return nil
		},
	}
	createCmd.Flags().StringVarP(&username, "username", "u", "", "Username (required)")
	createCmd.Flags().StringVarP(&password, "password", "p", "", "Password (required)")
	_ = createCmd.MarkFlagRequired("username")
	_ = createCmd.MarkFlagRequired("password")
	cmd.AddCommand(createCmd)
	return cmd
}

// ─── providers ────────────────────────────────────────────────────────────────

func providersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "providers", Short: "Manage providers"}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List providers",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, _, provStore, cleanup := mustPool()
			defer cleanup()
			list, err := provStore.List(context.Background(), false)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(list)
		},
	})
	return cmd
}
