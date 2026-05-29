package configloader

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/cmdexec"
	"github.com/haloydev/haloy/internal/config"
)

func TestFetchFrom1Password_BuildsCommandAndParsesFields(t *testing.T) {
	var gotOpts cmdexec.CLICommandOptions
	var gotName string
	var gotArgs []string

	with1PasswordProviderTestDouble(t, func(_ context.Context, opts cmdexec.CLICommandOptions, name string, args ...string) (string, error) {
		gotOpts = opts
		gotName = name
		gotArgs = append([]string(nil), args...)
		return `{"fields":[{"label":"username","value":"alice"},{"label":"password","value":"secret"}]}`, nil
	})

	got, err := fetchFrom1Password(context.Background(), config.OnePasswordSourceConfig{
		Account: "work",
		Vault:   "apps",
		Item:    "production",
	})
	if err != nil {
		t.Fatalf("fetchFrom1Password() returned error: %v", err)
	}

	expectedSecrets := map[string]string{
		"username": "alice",
		"password": "secret",
	}
	if !reflect.DeepEqual(got, expectedSecrets) {
		t.Fatalf("fetchFrom1Password() = %#v, want %#v", got, expectedSecrets)
	}

	if gotName != "op" {
		t.Fatalf("command name = %q, want op", gotName)
	}
	expectedArgs := []string{"item", "get", "production", "--vault", "apps", "--format", "json", "--account", "work"}
	if !reflect.DeepEqual(gotArgs, expectedArgs) {
		t.Fatalf("command args = %#v, want %#v", gotArgs, expectedArgs)
	}
	if gotOpts.WaitMessage != onePasswordWaitMessage {
		t.Fatalf("wait message = %q, want %q", gotOpts.WaitMessage, onePasswordWaitMessage)
	}
	if strings.Contains(gotOpts.WaitMessage, "apps") || strings.Contains(gotOpts.WaitMessage, "production") {
		t.Fatalf("wait message includes secret source metadata: %q", gotOpts.WaitMessage)
	}
}

func TestFetchFrom1Password_PropagatesCommandErrors(t *testing.T) {
	with1PasswordProviderTestDouble(t, func(_ context.Context, _ cmdexec.CLICommandOptions, _ string, _ ...string) (string, error) {
		return "", errors.New("op failed")
	})

	_, err := fetchFrom1Password(context.Background(), config.OnePasswordSourceConfig{
		Vault: "apps",
		Item:  "production",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "op failed") {
		t.Fatalf("error = %q, want command error", err.Error())
	}
}

func with1PasswordProviderTestDouble(t *testing.T, runner func(context.Context, cmdexec.CLICommandOptions, string, ...string) (string, error)) {
	t.Helper()

	original := run1PasswordCLICommand
	run1PasswordCLICommand = runner
	t.Cleanup(func() {
		run1PasswordCLICommand = original
	})
}
