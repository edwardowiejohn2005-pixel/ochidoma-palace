// Command createadmin bootstraps the very first super_admin user.
// Run once after migrations, before the API has any admins:
//
//	go run ./cmd/createadmin -email you@example.com -name "Your Name"
//
// It prompts for a password on stdin rather than accepting it as a flag,
// so it never ends up in shell history.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"golang.org/x/term"

	"github.com/ochidoma/platform/internal/auth"
	"github.com/ochidoma/platform/internal/config"
	"github.com/ochidoma/platform/internal/db"
	"github.com/ochidoma/platform/internal/repository"
)

func main() {
	email := flag.String("email", "", "admin email address")
	name := flag.String("name", "", "admin full name")
	role := flag.String("role", "super_admin", "role: super_admin | palace_editor | palace_publisher | cultural_editor")
	flag.Parse()

	if *email == "" || *name == "" {
		fmt.Println("usage: go run ./cmd/createadmin -email you@example.com -name \"Your Name\" [-role super_admin]")
		os.Exit(1)
	}

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fatal("config error: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("database error: %v", err)
	}
	defer pool.Close()

	users := repository.NewUserRepo(pool)

	existing, err := users.GetByEmail(ctx, *email)
	if err != nil {
		fatal("lookup error: %v", err)
	}
	if existing != nil {
		fatal("a user with email %s already exists", *email)
	}

	roleID, err := users.RoleIDByName(ctx, *role)
	if err != nil {
		fatal("unknown role %q: %v", *role, err)
	}

	password := readPasswordTwice()
	if len(password) < 12 {
		fatal("password must be at least 12 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fatal("hashing error: %v", err)
	}

	user, err := users.Create(ctx, *email, hash, *name, roleID)
	if err != nil {
		fatal("create error: %v", err)
	}

	fmt.Printf("\nCreated %s (%s) with role %s. You can now log in via POST /api/auth/login.\n",
		user.Email, user.ID, *role)
}

func readPasswordTwice() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Set password (input hidden): ")
		p1, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fatal("could not read password: %v", err)
		}
		fmt.Print("Confirm password: ")
		p2, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fatal("could not read password: %v", err)
		}
		if strings.TrimSpace(string(p1)) != strings.TrimSpace(string(p2)) {
			fmt.Println("passwords did not match, try again")
			continue
		}
		_ = reader // reserved if we later want a non-hidden fallback path
		return strings.TrimSpace(string(p1))
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
