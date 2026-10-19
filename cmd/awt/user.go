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

	"golang.org/x/term"

	"awt/internal/auth"
	"awt/internal/config"
	"awt/internal/store"
)

const userUsage = "usage: awt user add --email E --name N --role admin|scorer | passwd --email E | assign --email E --competition ID|CODE | reset-mfa --email E (password on stdin for add and passwd)"

// userCmd manages accounts: `awt user add|passwd|assign|reset-mfa`.
func userCmd(args []string, stdin *os.File) error {
	if len(args) == 0 {
		return errors.New(userUsage)
	}
	fs := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
	email := fs.String("email", "", "account email")
	name := fs.String("name", "", "display name (add)")
	role := fs.String("role", "", "admin or scorer (add)")
	comp := fs.String("competition", "", "competition id or code (assign)")
	switch args[0] {
	case "add", "passwd", "assign", "reset-mfa":
	default:
		return errors.New(userUsage)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("--email is required")
	}
	switch {
	case args[0] == "add" && (*name == "" || *role == ""):
		return errors.New("--name and --role are required")
	case args[0] == "assign" && *comp == "":
		return errors.New("--competition is required")
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

	switch args[0] {
	case "add":
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
	case "reset-mfa":
		u, err := a.UserByEmail(ctx, *email)
		if err != nil {
			return err
		}
		if err := a.ResetMFA(ctx, u.ID, "cli", ""); err != nil {
			return err
		}
		fmt.Printf("second factors removed for %s; all sessions closed\n", u.Email)
	case "assign":
		if err := a.AssignCompetition(ctx, *email, *comp); err != nil {
			return err
		}
		fmt.Printf("%s assigned to %s\n", *email, *comp)
	}
	return nil
}

// readPassword reads one line from stdin, without echo when it is a terminal.
func readPassword(stdin *os.File) (string, error) {
	if term.IsTerminal(int(stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Password: ")
		b, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
