package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/spf13/cobra"

	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/database"
	"github.com/marcelo/devctl/internal/docker"
	"github.com/marcelo/devctl/internal/hosts"
	"github.com/marcelo/devctl/internal/ssl"
	"github.com/marcelo/devctl/internal/traefik"
)

func init() {
	initCmd.Flags().Bool("ssl", false, "Enable SSL (installs mkcert and generates certificates)")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize devctl (create proxy network, start Traefik)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg := config.DefaultConfig()
		enableSSL, _ := cmd.Flags().GetBool("ssl")

		fmt.Println("Initializing devctl...")

		// 1. Create directories
		fmt.Print("Creating directories... ")
		if err := cfg.EnsureDirs(); err != nil {
			return fmt.Errorf("creating directories: %w", err)
		}
		fmt.Println("done")

		// 1b. Copy templates to ~/.devctl/templates/
		fmt.Print("Copying templates... ")
		if err := copyTemplatesToDataDir(cfg); err != nil {
			fmt.Printf("warning: %v\n", err)
		} else {
			fmt.Println("done")
		}

		// 2. SSL setup (only when --ssl flag is used)
		if enableSSL {
			if !ssl.IsMkcertInstalled() {
				fmt.Print("Installing mkcert... ")
				if err := installMkcert(); err != nil {
					fmt.Printf("failed: %v\n", err)
					fmt.Println("  HTTPS certificates will not be generated. Install mkcert manually: https://github.com/FiloSottile/mkcert")
				} else {
					fmt.Println("done")
				}
			}

			if ssl.IsMkcertInstalled() {
				fmt.Print("Installing mkcert CA... ")
				sslMgr := ssl.NewManager(cfg.CertsDir)
				if err := sslMgr.InstallCA(); err != nil {
					fmt.Printf("warning: %v\n", err)
				} else {
					fmt.Println("done")
				}

				// WSL: browsers run on Windows, so the CA must be trusted there too
				if isWSL() {
					caRoot := getMkcertCARoot()
					if caRoot != "" {
						fmt.Print("Installing mkcert CA on Windows (UAC prompt)... ")
						if err := installCAOnWindows(caRoot); err != nil {
							fmt.Printf("skipped: %v\n", err)
							fmt.Println("  You can install it manually in PowerShell (Admin):")
							fmt.Printf("    wsl -d %s cat %s/rootCA.pem > %%TEMP%%\\\\rootCA.pem\n", getWSLDistro(), caRoot)
							fmt.Printf("    certutil -addstore Root %%TEMP%%\\rootCA.pem\n")
						} else {
							fmt.Println("done")
						}
					}
				}

				// Generate cert for devctl dashboard
				fmt.Print("Generating certificate for devctl.local... ")
				if _, _, err := sslMgr.GenerateCert("devctl.local"); err != nil {
					fmt.Printf("warning: %v\n", err)
				} else {
					fmt.Println("done")
				}
			}
		}

		// 3. Create Docker client
		dockerCli, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("connecting to Docker: %w", err)
		}
		defer dockerCli.Close()

		if !dockerCli.IsDockerRunning(ctx) {
			return fmt.Errorf("Docker daemon is not running. Please start Docker first")
		}

		// 4. Create proxy network
		fmt.Print("Creating devctl-proxy network... ")
		netMgr := docker.NewNetworkManager(dockerCli.Raw())
		if err := netMgr.EnsureProxyNetwork(ctx); err != nil {
			return fmt.Errorf("creating proxy network: %w", err)
		}
		fmt.Println("done")

		// 5. Open DB (for migrations)
		db, err := database.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer db.Close()

		// 6. Start Traefik
		fmt.Print("Starting Traefik... ")
		traefikMgr := traefik.NewManager(dockerCli.Raw(), cfg.TraefikDir, cfg.CertsDir)
		if err := traefikMgr.Start(ctx); err != nil {
			return fmt.Errorf("starting Traefik: %w", err)
		}
		fmt.Println("done")

		// 6. Configure passwordless sudo for /etc/hosts editing
		fmt.Print("Configuring sudo for /etc/hosts management... ")
		if err := configureSudoersForHosts(); err != nil {
			fmt.Printf("skipped: %v\n", err)
		} else {
			fmt.Println("done")
		}

		// 7. Add devctl.local to /etc/hosts
		fmt.Print("Adding devctl.local to /etc/hosts... ")
		hostsMgr := hosts.NewManager()
		if err := hostsMgr.AddDomain("devctl.local"); err != nil {
			fmt.Printf("failed: %v\n", err)
			fmt.Println("  You can add it manually: sudo sh -c 'echo \"127.0.0.1 devctl.local\" >> /etc/hosts'")
		} else {
			fmt.Println("done")
		}

		fmt.Println("\ndevctl initialized successfully!")
		scheme := "http"
		if enableSSL {
			scheme = "https"
		}
		fmt.Printf("Dashboard will be available at %s://devctl.local (run 'devctl serve' to start)\n", scheme)

		return nil
	},
}

// copyTemplatesToDataDir copies .yaml templates from the source directory
// (next to executable or CWD) into ~/.devctl/templates/.
// Only missing templates are copied — existing ones are preserved to keep
// user customizations intact.
func copyTemplatesToDataDir(cfg *config.Config) error {
	// Find source directory
	var srcDir string
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), "templates")
		if config.HasTemplates(candidate) {
			srcDir = candidate
		}
	}
	if srcDir == "" {
		if config.HasTemplates("templates") {
			srcDir = "templates"
		}
	}
	if srcDir == "" {
		return fmt.Errorf("no template source found")
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(cfg.TemplatesDir, 0755); err != nil {
		return err
	}

	// Copy only missing templates (don't overwrite user customizations)
	files, err := filepath.Glob(filepath.Join(srcDir, "*.yaml"))
	if err != nil {
		return err
	}
	for _, f := range files {
		dst := filepath.Join(cfg.TemplatesDir, filepath.Base(f))
		if _, err := os.Stat(dst); err == nil {
			continue // already exists, skip
		}
		if err := copyFile(f, dst); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := bytes.ToLower(data)
	return bytes.Contains(lower, []byte("microsoft")) || bytes.Contains(lower, []byte("wsl"))
}

func getWSLDistro() string {
	if d := os.Getenv("WSL_DISTRO_NAME"); d != "" {
		return d
	}
	return "Ubuntu"
}

func getMkcertCARoot() string {
	out, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// installMkcert installs mkcert using the system package manager or direct download.
func installMkcert() error {
	// Try apt (Debian/Ubuntu)
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("sudo", "apt-get", "update", "-qq")
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		cmd = exec.Command("sudo", "apt-get", "install", "-y", "-qq", "mkcert")
		cmd.Stderr = os.Stderr
		if cmd.Run() == nil {
			return nil
		}
	}

	// Fallback: download binary from GitHub
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	}
	url := fmt.Sprintf("https://github.com/FiloSottile/mkcert/releases/latest/download/mkcert-v1.4.4-linux-%s", arch)
	dest := "/usr/local/bin/mkcert"

	cmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("curl -fsSL %s -o %s && chmod +x %s", url, dest, dest))
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installCAOnWindows installs the mkcert root CA into the Windows certificate
// store using an elevated PowerShell process (triggers UAC prompt).
func installCAOnWindows(caRoot string) error {
	caPEM, err := os.ReadFile(filepath.Join(caRoot, "rootCA.pem"))
	if err != nil {
		return fmt.Errorf("reading CA cert: %w", err)
	}

	pemB64 := base64.StdEncoding.EncodeToString(caPEM)

	// PowerShell script: decode the PEM to a temp file, import it, clean up
	psScript := fmt.Sprintf(
		"$bytes = [Convert]::FromBase64String('%s'); "+
			"$tmp = [IO.Path]::GetTempFileName() + '.pem'; "+
			"[IO.File]::WriteAllBytes($tmp, $bytes); "+
			"certutil -addstore Root $tmp; "+
			"Remove-Item $tmp",
		pemB64,
	)

	encodedCmd := psToEncodedCommand(psScript)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command",
		fmt.Sprintf(
			`Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-EncodedCommand','%s'`,
			encodedCmd,
		),
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// psToEncodedCommand converts a PowerShell script to base64-encoded UTF-16LE
// for use with PowerShell's -EncodedCommand parameter.
func psToEncodedCommand(script string) string {
	runes := utf16.Encode([]rune(script))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// configureSudoersForHosts creates a sudoers drop-in rule so devctl can
// edit /etc/hosts without a password prompt (needed when running as a service).
func configureSudoersForHosts() error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	rule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: /usr/bin/tee /etc/hosts\n", u.Username)
	sudoersFile := "/etc/sudoers.d/devctl-hosts"

	// Check if already configured
	existing, _ := os.ReadFile(sudoersFile)
	if string(existing) == rule {
		return nil
	}

	cmd := exec.Command("sudo", "tee", sudoersFile)
	cmd.Stdin = bytes.NewBufferString(rule)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// sudoers files must be 0440
	chmodCmd := exec.Command("sudo", "chmod", "0440", sudoersFile)
	chmodCmd.Stderr = os.Stderr
	return chmodCmd.Run()
}
