package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"awt/internal/auth"
	"awt/internal/config"
	"awt/internal/store"
)

const userUsage = "usage: awt user add --email E --name N --role admin|scorer | passwd --email E | assign --email E --competition ID|CODE (password on stdin)"

// userCmd manages accounts: `awt user add|passwd|assign`.
func userCmd(args []string, stdin io.Reader) error {
	if len(args) == 0 {
		return errors.New(userUsage)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	a := auth.New(db, cfg)
	ctx := context.Background()

	fs := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
	email := fs.String("email", "", "account email")
	name := fs.String("name", "", "display name (add)")
	role := fs.String("role", "", "admin or scorer (add)")
	comp := fs.String("competition", "", "competition id or code (assign)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("--email is required")
	}
	switch args[0] {
	case "add":
		if *name == "" || *role == "" {
			return errors.New("--name and --role are required")
		}
		pw, err := readPassword(stdin)
		if err != nil {
			return err
		}
		u, err := a.CreateUser(ctx, *email, *name, *role, pw)
		if err != nil {
			return err
		}
		fmt.Printf("created %s (%s) id %s\n", u.Email, u.Role, u.ID)
	case "passwd":
		pw, err := readPassword(stdin)
		if err != nil {
			return err
		}
		if err := a.SetPassword(ctx, *email, pw); err != nil {
			return err
		}
		fmt.Printf("password updated for %s\n", *email)
	case "assign":
		if *comp == "" {
			return errors.New("--competition is required")
		}
		if err := a.AssignCompetition(ctx, *email, *comp); err != nil {
			return err
		}
		fmt.Printf("%s assigned to %s\n", *email, *comp)
	default:
		return errors.New(userUsage)
	}
	return nil
}

// readPassword reads one line from r (no echo handling).
func readPassword(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	pw := strings.TrimRight(line, "\r\n")
	if pw == "" {
		return "", errors.New("empty password")
	}
	return pw, nil
}
