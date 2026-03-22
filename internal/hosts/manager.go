package hosts

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf16"
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
// On WSL it also updates the Windows hosts file so browsers resolve the domains.
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
		} else {
			return fmt.Errorf("writing hosts file: %w", err)
		}
	}

	// On WSL, also sync to the Windows hosts file
	if isWSL() {
		fmt.Fprintln(os.Stderr, "[devctl] Atualizando hosts do Windows — uma janela UAC pode aparecer, clique em Sim.")
		if err := syncToWindowsHosts(domains); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ Não foi possível atualizar o hosts do Windows: %v\n", err)
			fmt.Fprintln(os.Stderr, "  As entradas serão perdidas ao reiniciar o WSL. Execute 'devctl hosts-sync' para tentar novamente.")
		}
	}

	return nil
}

// writeWithSudo writes content to the hosts file using sudo tee.
// Content is staged in a temp file so that stdin stays free for the sudo
// password prompt when running interactively (avoids WSL2 stdin collision).
func (m *Manager) writeWithSudo(content string) error {
	// Stage content in a temp file
	tmp, err := os.CreateTemp("", "devctl-hosts-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// Try non-interactive first (works if NOPASSWD sudoers rule is configured)
	f, err := os.Open(tmp.Name())
	if err != nil {
		return err
	}
	cmd := exec.Command("sudo", "-n", "tee", m.filePath)
	cmd.Stdin = f
	cmd.Stdout = io.Discard
	cmd.Stderr = nil
	runErr := cmd.Run()
	f.Close()
	if runErr == nil {
		return nil
	}

	// Fall back to interactive sudo: pipe content via shell redirect so stdin
	// remains available for the password prompt (avoids stdin collision on WSL2)
	cmd = exec.Command("sudo", "sh", "-c",
		fmt.Sprintf("tee %s < %s > /dev/null", m.filePath, tmp.Name()))
	cmd.Stdin = os.Stdin
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

// isWSL returns true when running inside Windows Subsystem for Linux.
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := bytes.ToLower(data)
	return bytes.Contains(lower, []byte("microsoft")) || bytes.Contains(lower, []byte("wsl"))
}

const windowsHostsPath = "/mnt/c/Windows/System32/drivers/etc/hosts"

// syncToWindowsHosts updates the Windows hosts file with the same managed block.
// It reads the current Windows hosts file, replaces the managed block, and writes
// it back using PowerShell with elevation (shows a UAC prompt).
func syncToWindowsHosts(domains []string) error {
	content, err := os.ReadFile(windowsHostsPath)
	if err != nil {
		return fmt.Errorf("reading Windows hosts: %w", err)
	}

	// Preserve UTF-8 BOM if present
	raw := string(content)
	bom := ""
	if strings.HasPrefix(raw, "\xef\xbb\xbf") {
		bom = "\xef\xbb\xbf"
		raw = raw[3:]
	}

	before, _, after := splitManagedBlock(raw)

	var block strings.Builder
	block.WriteString(startMarker + "\n")
	for _, d := range domains {
		block.WriteString(fmt.Sprintf("127.0.0.1 %s\n", d))
	}
	block.WriteString(endMarker + "\n")

	newContent := bom + before + block.String() + after

	// Try direct write first (may work depending on WSL mount permissions)
	if err := os.WriteFile(windowsHostsPath, []byte(newContent), 0644); err == nil {
		return nil
	}

	// Fall back to PowerShell with elevation
	return writeWindowsHostsElevated(newContent)
}

// writeWindowsHostsElevated writes content to the Windows hosts file using
// an elevated PowerShell process (triggers UAC prompt).
// It uses -EncodedCommand to avoid temp files and path/quoting issues.
func writeWindowsHostsElevated(content string) error {
	// Build the PowerShell script that writes the hosts file
	contentB64 := base64.StdEncoding.EncodeToString([]byte(content))
	psScript := fmt.Sprintf(
		"[IO.File]::WriteAllBytes('C:\\Windows\\System32\\drivers\\etc\\hosts',"+
			"[Convert]::FromBase64String('%s'))",
		contentB64,
	)

	// PowerShell -EncodedCommand requires UTF-16LE base64
	encodedCmd := toEncodedCommand(psScript)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command",
		fmt.Sprintf(
			`Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-EncodedCommand','%s'`,
			encodedCmd,
		),
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// toEncodedCommand converts a PowerShell script string to the base64-encoded
// UTF-16LE format required by PowerShell's -EncodedCommand parameter.
func toEncodedCommand(script string) string {
	runes := utf16.Encode([]rune(script))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
