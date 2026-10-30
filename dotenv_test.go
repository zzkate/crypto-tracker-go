package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# комментарий\nDOTENV_TEST_A=alpha\nDOTENV_TEST_B=\"quoted value\"\nDOTENV_TEST_C='single'\nEMPTY_LINE=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	loadDotEnv(path)

	if os.Getenv("DOTENV_TEST_A") != "alpha" {
		t.Fatalf("A = %q", os.Getenv("DOTENV_TEST_A"))
	}
	if os.Getenv("DOTENV_TEST_B") != "quoted value" {
		t.Fatalf("B = %q", os.Getenv("DOTENV_TEST_B"))
	}
	if os.Getenv("DOTENV_TEST_C") != "single" {
		t.Fatalf("C = %q", os.Getenv("DOTENV_TEST_C"))
	}
}

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("DOTENV_TEST_KEEP=from-file\n"), 0o600)

	os.Setenv("DOTENV_TEST_KEEP", "from-env")
	loadDotEnv(path)

	if os.Getenv("DOTENV_TEST_KEEP") != "from-env" {
		t.Fatalf("env should win, got %q", os.Getenv("DOTENV_TEST_KEEP"))
	}
	os.Unsetenv("DOTENV_TEST_KEEP")
}

func TestLoadDotEnvMissingFile(t *testing.T) {
	loadDotEnv("/nonexistent/.env") // не должно падать
}
