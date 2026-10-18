package main

import (
	"strings"
	"testing"
)

func TestUserCmdValidatesArgsBeforeOpeningDB(t *testing.T) {
	t.Setenv("AWT_DB_PATH", "/nonexistent-dir/awt.db") // would fail if opened
	cases := map[string][]string{
		"no subcommand":       {},
		"unknown subcommand":  {"frobnicate", "--email", "a@b.c"},
		"missing email":       {"add", "--name", "N", "--role", "admin"},
		"missing role":        {"add", "--email", "a@b.c", "--name", "N"},
		"missing competition": {"assign", "--email", "a@b.c"},
	}
	for name, args := range cases {
		err := userCmd(args, nil)
		if err == nil {
			t.Errorf("%s: expected an error", name)
		} else if strings.Contains(err.Error(), "nonexistent") || strings.Contains(err.Error(), "unable to open") {
			t.Errorf("%s: DB opened before validation: %v", name, err)
		}
	}
}
