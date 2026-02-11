package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestHosts(t *testing.T, content string) *Manager {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return NewManagerWithPath(path)
}

func TestSetDomains_CreatesBlock(t *testing.T) {
	m := setupTestHosts(t, "127.0.0.1 localhost\n")

	err := m.SetDomains([]string{"devctl.local", "projeto1.local"})
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(m.filePath)
	s := string(content)

	if !strings.Contains(s, startMarker) {
		t.Error("missing start marker")
	}
	if !strings.Contains(s, endMarker) {
		t.Error("missing end marker")
	}
	if !strings.Contains(s, "127.0.0.1 devctl.local") {
		t.Error("missing devctl.local entry")
	}
	if !strings.Contains(s, "127.0.0.1 projeto1.local") {
		t.Error("missing projeto1.local entry")
	}
	if !strings.Contains(s, "127.0.0.1 localhost") {
		t.Error("original content was modified")
	}
}

func TestSetDomains_ReplacesExistingBlock(t *testing.T) {
	initial := "127.0.0.1 localhost\n" +
		startMarker + "\n" +
		"127.0.0.1 old.local\n" +
		endMarker + "\n"

	m := setupTestHosts(t, initial)

	err := m.SetDomains([]string{"new.local"})
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(m.filePath)
	s := string(content)

	if strings.Contains(s, "old.local") {
		t.Error("old domain should have been removed")
	}
	if !strings.Contains(s, "127.0.0.1 new.local") {
		t.Error("missing new.local entry")
	}
}

func TestAddDomain(t *testing.T) {
	m := setupTestHosts(t, "127.0.0.1 localhost\n")

	m.SetDomains([]string{"a.local"})
	m.AddDomain("b.local")

	domains, err := m.GetDomains()
	if err != nil {
		t.Fatal(err)
	}

	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}
}

func TestAddDomain_NoDuplicate(t *testing.T) {
	m := setupTestHosts(t, "127.0.0.1 localhost\n")

	m.SetDomains([]string{"a.local"})
	m.AddDomain("a.local")

	domains, _ := m.GetDomains()
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain (no duplicate), got %d", len(domains))
	}
}

func TestRemoveDomain(t *testing.T) {
	m := setupTestHosts(t, "127.0.0.1 localhost\n")

	m.SetDomains([]string{"a.local", "b.local", "c.local"})
	m.RemoveDomain("b.local")

	domains, _ := m.GetDomains()
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}
	for _, d := range domains {
		if d == "b.local" {
			t.Error("b.local should have been removed")
		}
	}
}

func TestGetDomains_EmptyFile(t *testing.T) {
	m := setupTestHosts(t, "")

	domains, err := m.GetDomains()
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(domains))
	}
}

func TestPreservesContentOutsideBlock(t *testing.T) {
	initial := "127.0.0.1 localhost\n::1 localhost\n"
	m := setupTestHosts(t, initial)

	m.SetDomains([]string{"test.local"})

	content, _ := os.ReadFile(m.filePath)
	s := string(content)

	if !strings.Contains(s, "127.0.0.1 localhost") {
		t.Error("lost content before block")
	}
	if !strings.Contains(s, "::1 localhost") {
		t.Error("lost content before block")
	}
}
