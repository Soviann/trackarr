package cmd

import (
	"context"
	"crypto/rand"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func ResetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	passwordFlag := fs.String("password", "", "New admin password")
	usernameFlag := fs.String("username", "", "Admin username (default: admin)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := database.ResolveDBPath(cfg.DataDir)
	writeDB, _, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer writeDB.Close()

	if err := database.Migrate(writeDB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	settingRepo := repository.NewSettingRepository(writeDB)

	password := *passwordFlag
	if password == "" {
		if envPass := os.Getenv("TRACKARR_ADMIN_PASSWORD"); envPass != "" {
			password = envPass
		} else {
			fmt.Print("Enter new admin password (min 8 chars): ")
			var p string
			if _, err := fmt.Scanln(&p); err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			password = strings.TrimSpace(p)
		}
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	username := *usernameFlag
	if username == "" {
		if existing, err := settingRepo.Get("admin_username"); err == nil && existing != "" {
			username = existing
		} else {
			username = "admin"
		}
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Generate emergency recovery key
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	chars := make([]byte, 12)
	for i := 0; i < 12; i++ {
		chars[i] = alphabet[int(b[i])%len(alphabet)]
	}
	recKeyFormatted := fmt.Sprintf("TRCK-%s-%s-%s", string(chars[0:4]), string(chars[4:8]), string(chars[8:12]))
	recKeyNorm := "TRCK" + string(chars)

	recKeyHash, err := bcrypt.GenerateFromPassword([]byte(recKeyNorm), 12)
	if err != nil {
		return fmt.Errorf("hash recovery key: %w", err)
	}

	if err := database.WithTxContext(context.Background(), writeDB, func(tx *sql.Tx) error {
		w := repository.NewSettingWriter(tx)
		if err := w.Set(context.Background(), "admin_username", username); err != nil {
			return err
		}
		if err := w.Set(context.Background(), "admin_password_hash", string(passHash)); err != nil {
			return err
		}
		return w.Set(context.Background(), "admin_recovery_key_hash", string(recKeyHash))
	}); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}

	fmt.Println("\n✅ Admin password successfully updated!")
	fmt.Printf("   Username: %s\n", username)
	fmt.Println("\n⚠️  NEW EMERGENCY RECOVERY KEY (SAVE IT SECURELY):")
	fmt.Printf("   %s\n", recKeyFormatted)

	return nil
}
