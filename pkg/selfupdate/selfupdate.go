package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/rexadb/rexadb/pkg/output"
)

const (
	owner       = "rexadbapp"
	repo        = "rexadb"
	binName     = "rexadb"
)

var (
	currentVer  string
	currentVerMu sync.RWMutex
)

func SetVersion(v string) {
	currentVerMu.Lock()
	defer currentVerMu.Unlock()
	currentVer = v
}

func getVersion() string {
	currentVerMu.RLock()
	defer currentVerMu.RUnlock()
	return currentVer
}

type Release struct {
	TagName string `json:"tag_name"`
	Name   string `json:"name"`
	Body   string `json:"body"`
	Assets []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	BrowserURL  string `json:"browser_download_url"`
	ContentType string `json:"content_type"`
}

func CheckForUpdate() (bool, string, string) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Warning: failed to fetch latest version: %v\n", err)
		return false, "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Warning: failed to fetch latest version (status %d)\n", resp.StatusCode)
		return false, "", ""
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Printf("Warning: failed to decode release info: %v\n", err)
		return false, "", ""
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	if latestVer != getVersion() {
		return true, latestVer, release.Body
	}
	return false, latestVer, ""
}

func PrintUpdateNotice(newVer, notes string) {
	output.Println()
	output.Printf("  %s\n", output.Yellow("⚠ New version available: v"+newVer))
	output.Println()
	if notes != "" {
		lines := strings.Split(notes, "\n")
		for _, line := range lines {
			if line == "" || strings.HasPrefix(line, "##") {
				continue
			}
			output.Printf("    %s\n", line)
		}
	}
	output.Println()
	output.Println("  To update, run: rexadb update")
	output.Println()
}

func Update() error {
	hasUpdate, newVer, _ := CheckForUpdate()
	if !hasUpdate {
		output.Println(output.Green("✓ Already on latest version: v" + getVersion()))
		return nil
	}

	output.Printf("Updating from v%s to v%s...\n", getVersion(), newVer)

	binDir, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find executable: %w", err)
	}
	binDir = filepath.Dir(binDir)

	tmpDir, err := os.MkdirTemp("", "rexadb-update")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode release: %w", err)
	}

	arch := runtime.GOARCH
	osName := runtime.GOOS
	ext := ""
	if osName == "windows" {
		ext = ".exe"
	}

	underscoreName := fmt.Sprintf("%s_%s_%s%s", binName, osName, arch, ext)
	hyphenName := fmt.Sprintf("%s-%s-%s%s", binName, osName, arch, ext)

	var assetURL string
	for _, a := range release.Assets {
		if a.Name == underscoreName || a.Name == hyphenName {
			assetURL = a.BrowserURL
			break
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no compatible binary found for %s/%s", osName, arch)
	}

	binPath := filepath.Join(tmpDir, binName+ext)
	if err := downloadFile(assetURL, binPath); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	newPath := filepath.Join(tmpDir, "rexadb-new"+ext)
	if err := os.Rename(binPath, newPath); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	currentBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find current binary: %w", err)
	}

	if err := os.Rename(currentBin, currentBin+".old"); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(newPath, currentBin); err != nil {
		os.Rename(currentBin+".old", currentBin)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	output.Println(output.Green("✓ Update complete. Restarting..."))

	cmd := exec.Command(currentBin, "--version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	return nil
}

func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch release (status %d)", resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.ReadFrom(resp.Body)
	return err
}