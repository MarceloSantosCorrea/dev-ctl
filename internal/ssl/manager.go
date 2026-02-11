package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Manager struct {
	certsDir string
}

func NewManager(certsDir string) *Manager {
	return &Manager{certsDir: certsDir}
}

// InstallCA installs the mkcert root CA into the system trust store.
func (m *Manager) InstallCA() error {
	cmd := exec.Command("mkcert", "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GenerateCert generates a TLS certificate for the given domain using mkcert.
func (m *Manager) GenerateCert(domain string) (certPath, keyPath string, err error) {
	if err := os.MkdirAll(m.certsDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating certs directory: %w", err)
	}

	certPath = filepath.Join(m.certsDir, domain+".pem")
	keyPath = filepath.Join(m.certsDir, domain+"-key.pem")

	// Skip if cert already exists
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return certPath, keyPath, nil
		}
	}

	cmd := exec.Command("mkcert",
		"-cert-file", certPath,
		"-key-file", keyPath,
		domain,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("generating certificate for %s: %w", domain, err)
	}

	return certPath, keyPath, nil
}

// CertExists checks if the certificate files for a domain exist.
func (m *Manager) CertExists(domain string) bool {
	certPath := filepath.Join(m.certsDir, domain+".pem")
	keyPath := filepath.Join(m.certsDir, domain+"-key.pem")

	if _, err := os.Stat(certPath); err != nil {
		return false
	}
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	return true
}

// RemoveCert removes the certificate files for a domain.
func (m *Manager) RemoveCert(domain string) error {
	certPath := filepath.Join(m.certsDir, domain+".pem")
	keyPath := filepath.Join(m.certsDir, domain+"-key.pem")
	os.Remove(certPath)
	os.Remove(keyPath)
	return nil
}

// IsMkcertInstalled checks if mkcert is available on the system.
func IsMkcertInstalled() bool {
	_, err := exec.LookPath("mkcert")
	return err == nil
}
