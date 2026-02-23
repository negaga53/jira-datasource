//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Build builds the backend plugin binary for the current platform.
func Build() error {
	return buildFor(runtime.GOOS, runtime.GOARCH)
}

// BuildAll builds for Linux (amd64 and arm64), Darwin (amd64 and arm64), and Windows (amd64).
func BuildAll() error {
	targets := []struct{ os, arch string }{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}
	for _, t := range targets {
		if err := buildFor(t.os, t.arch); err != nil {
			return err
		}
	}
	return nil
}

func buildFor(targetOS, targetArch string) error {
	ext := ""
	if targetOS == "windows" {
		ext = ".exe"
	}
	output := filepath.Join("dist", fmt.Sprintf("gpx_jira-datasource_%s_%s%s", targetOS, targetArch, ext))

	fmt.Printf("Building %s/%s -> %s\n", targetOS, targetArch, output)
	cmd := exec.Command("go", "build", "-o", output, "-ldflags", "-w -s", "./pkg")
	cmd.Env = append(os.Environ(),
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
		"CGO_ENABLED=0",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Clean removes the dist directory.
func Clean() error {
	return os.RemoveAll("dist")
}
