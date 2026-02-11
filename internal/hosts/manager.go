package hosts

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	startMarker = "# >>> DEVCTL MANAGED - DO NOT EDIT >>>"
	endMarker   = "# <<< DEVCTL MANAGED <<<"
	hostsFile   = "/etc/hosts"
)

type Manager struct {
	filePath string
}

func NewManager() *Manager {
	return &Manager{filePath: hostsFile}
}

// NewManagerWithPath creates a manager pointing at a custom hosts file (for testing).
func NewManagerWithPath(path string) *Manager {
	return &Manager{filePath: path}
}

// SetDomains replaces the entire managed block with the given list of domains.
func (m *Manager) SetDomains(domains []string) error {
	content, err := os.ReadFile(m.filePath)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	before, _, after := splitManagedBlock(string(content))

	var block strings.Builder
	block.WriteString(startMarker + "\n")
	for _, d := range domains {
		block.WriteString(fmt.Sprintf("127.0.0.1 %s\n", d))
	}
	block.WriteString(endMarker + "\n")

	newContent := before + block.String() + after

	if err := os.WriteFile(m.filePath, []byte(newContent), 0644); err != nil {
		if errors.Is(err, os.ErrPermission) {
			if sudoErr := m.writeWithSudo(newContent); sudoErr != nil {
				return fmt.Errorf("writing hosts file (tried sudo): %w", sudoErr)
			}
			return nil
		}
		return fmt.Errorf("writing hosts file: %w", err)
	}

	return nil
}

// writeWithSudo writes content to the hosts file using sudo tee.
func (m *Manager) writeWithSudo(content string) error {
	cmd := exec.Command("sudo", "tee", m.filePath)
	cmd.Stdin = bytes.NewBufferString(content)
	cmd.Stdout = nil // discard tee stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AddDomain adds a domain to the managed block if it doesn't already exist.
func (m *Manager) AddDomain(domain string) error {
	existing, err := m.GetDomains()
	if err != nil {
		return err
	}

	for _, d := range existing {
		if d == domain {
			return nil // already exists
		}
	}

	return m.SetDomains(append(existing, domain))
}

// RemoveDomain removes a domain from the managed block.
func (m *Manager) RemoveDomain(domain string) error {
	existing, err := m.GetDomains()
	if err != nil {
		return err
	}

	var filtered []string
	for _, d := range existing {
		if d != domain {
			filtered = append(filtered, d)
		}
	}

	return m.SetDomains(filtered)
}

// GetDomains returns the list of domains currently in the managed block.
func (m *Manager) GetDomains() ([]string, error) {
	content, err := os.ReadFile(m.filePath)
	if err != nil {
		return nil, fmt.Errorf("reading hosts file: %w", err)
	}

	_, managed, _ := splitManagedBlock(string(content))

	var domains []string
	for _, line := range strings.Split(managed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			domains = append(domains, parts[1])
		}
	}

	return domains, nil
}

// splitManagedBlock splits the hosts file into three parts:
// content before the block, the block content (without markers), and content after the block.
func splitManagedBlock(content string) (before, managed, after string) {
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return content, "", ""
	}

	before = content[:startIdx]
	managed = content[startIdx+len(startMarker)+1 : endIdx]
	after = content[endIdx+len(endMarker):]
	if len(after) > 0 && after[0] == '\n' {
		after = after[1:]
	}

	return before, managed, after
}
