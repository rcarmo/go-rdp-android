package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckVersionAlignmentAcceptsAlignedVersion(t *testing.T) {
	withTempProject(t, "1.2.3", `android {
    defaultConfig {
        versionCode = 42
        versionName = "1.2.3"
    }
}
`)

	if err := checkVersionAlignment(); err != nil {
		t.Fatalf("checkVersionAlignment() error = %v", err)
	}
}

func TestCheckVersionAlignmentRejectsMismatchedVersionName(t *testing.T) {
	withTempProject(t, "1.2.3", `android {
    defaultConfig {
        versionCode = 42
        versionName = "1.2.4"
    }
}
`)

	if err := checkVersionAlignment(); err == nil {
		t.Fatal("checkVersionAlignment() error = nil, want mismatch error")
	}
}

func TestCheckVersionAlignmentRequiresVersionCode(t *testing.T) {
	withTempProject(t, "1.2.3", `android {
    defaultConfig {
        versionName = "1.2.3"
    }
}
`)

	if err := checkVersionAlignment(); err == nil {
		t.Fatal("checkVersionAlignment() error = nil, want missing versionCode error")
	}
}

func TestFirstSubmatch(t *testing.T) {
	got := firstSubmatch(`versionName = "0.1.0"`, `versionName\s*=\s*"([^"]+)"`)
	if got != "0.1.0" {
		t.Fatalf("firstSubmatch() = %q, want %q", got, "0.1.0")
	}

	got = firstSubmatch(`versionName = "0.1.0"`, `versionCode\s*=\s*(\d+)`)
	if got != "" {
		t.Fatalf("firstSubmatch() = %q, want empty string", got)
	}
}

func withTempProject(t *testing.T, version, gradle string) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	gradlePath := filepath.Join(dir, "android", "app", "build.gradle.kts")
	if err := os.MkdirAll(filepath.Dir(gradlePath), 0o755); err != nil {
		t.Fatalf("mkdir gradle path: %v", err)
	}
	if err := os.WriteFile(gradlePath, []byte(gradle), 0o644); err != nil {
		t.Fatalf("write Gradle file: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})
}
