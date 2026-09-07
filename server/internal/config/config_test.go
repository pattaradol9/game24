package config

import (
	"os"
	"testing"
)

// writeFile creates a .env (or any file) inside the test's working directory.
func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// unsetEnv makes key look unset for the duration of the test and restores
// whatever was there before (tests share one process environment).
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, orig)
		}
	})
}

func TestLoadDotenvADCFileValueWinsOverInheritedEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(adcEnv, "/shell/creds.json")
	writeFile(t, ".env", adcEnv+"=/dotenv/creds.json\n")

	loadDotenv()

	if got := os.Getenv(adcEnv); got != "/dotenv/creds.json" {
		t.Fatalf("adcEnv = %q, want .env value %q", got, "/dotenv/creds.json")
	}
}

func TestLoadDotenvADCInheritedEnvWinsWhenFileValueEmpty(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(adcEnv, "/shell/creds.json")
	writeFile(t, ".env", adcEnv+"=\n")

	loadDotenv()

	if got := os.Getenv(adcEnv); got != "/shell/creds.json" {
		t.Fatalf("adcEnv = %q, want inherited value %q", got, "/shell/creds.json")
	}
}

func TestLoadDotenvFirstMatchingFileWins(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp+"/.env", adcEnv+"=/parent/creds.json\n")
	if err := os.MkdirAll(tmp+"/nested", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, tmp+"/nested/.env", adcEnv+"=/child/creds.json\n")
	t.Chdir(tmp)
	t.Chdir(tmp + "/nested")

	loadDotenv()

	if got := os.Getenv(adcEnv); got != "/child/creds.json" {
		t.Fatalf("adcEnv = %q, want nearest-file value %q", got, "/child/creds.json")
	}
}

func TestLoadDotenvOtherKeysInheritedEnvStillWins(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ENCRYPTION_KEY", "from-env")
	writeFile(t, ".env", "ENCRYPTION_KEY=from-file\n")

	loadDotenv()

	if got := os.Getenv("ENCRYPTION_KEY"); got != "from-env" {
		t.Fatalf("ENCRYPTION_KEY = %q, want inherited value %q", got, "from-env")
	}
}

func TestLoadDotenvMissingKeysComeFromTheFile(t *testing.T) {
	t.Chdir(t.TempDir())
	unsetEnv(t, "GOOGLE_OAUTH_CLIENT_ID")
	writeFile(t, ".env", "GOOGLE_OAUTH_CLIENT_ID=client-from-file\n")

	loadDotenv()

	if got := os.Getenv("GOOGLE_OAUTH_CLIENT_ID"); got != "client-from-file" {
		t.Fatalf("GOOGLE_OAUTH_CLIENT_ID = %q, want file value %q", got, "client-from-file")
	}
}

func TestLoadDotenvUnparsableFileChangesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(adcEnv, "/shell/creds.json")
	writeFile(t, ".env", "this is not\nvalid=\"env\n")

	loadDotenv()

	if got := os.Getenv(adcEnv); got != "/shell/creds.json" {
		t.Fatalf("adcEnv = %q, want inherited value %q", got, "/shell/creds.json")
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	for _, tc := range []struct{ in, want string }{
		{"~/creds.json", "/home/tester/creds.json"},
		{"~", "/home/tester"},
		{"/abs/creds.json", "/abs/creds.json"},
		{"relative/creds.json", "relative/creds.json"},
		{"~other/creds.json", "~other/creds.json"},
	} {
		if got := expandHome(tc.in); got != tc.want {
			t.Errorf("expandHome(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The .env search must be relative to the process working directory, not the
// package directory, so guard against accidental non-hermetic lookups.
func TestLoadDotenvSearchesWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	unsetEnv(t, adcEnv)
	writeFile(t, ".env", adcEnv+"=/tmp/creds.json\n")

	loadDotenv()

	if got := os.Getenv(adcEnv); got != "/tmp/creds.json" {
		t.Fatalf("adcEnv = %q, want file value %q", got, "/tmp/creds.json")
	}
}
