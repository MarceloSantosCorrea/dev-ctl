package ports

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}

	migrations := []string{
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			domain TEXT UNIQUE NOT NULL,
			status TEXT DEFAULT 'stopped',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE services (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			template_name TEXT NOT NULL,
			name TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			config TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE port_allocations (
			id TEXT PRIMARY KEY,
			service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
			internal_port INTEGER NOT NULL,
			external_port INTEGER NOT NULL,
			protocol TEXT DEFAULT 'tcp',
			UNIQUE(external_port, protocol)
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			t.Fatal(err)
		}
	}

	// Insert test project and services
	_, err = db.Exec("INSERT INTO projects (id, name, domain) VALUES ('p1', 'project1', 'project1.local')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO services (id, project_id, template_name, name) VALUES ('s1', 'p1', 'mysql', 'mysql')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO services (id, project_id, template_name, name) VALUES ('s2', 'p1', 'redis', 'redis')")
	if err != nil {
		t.Fatal(err)
	}

	// Second project
	_, err = db.Exec("INSERT INTO projects (id, name, domain) VALUES ('p2', 'project2', 'project2.local')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO services (id, project_id, template_name, name) VALUES ('s3', 'p2', 'mysql', 'mysql')")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})

	return db
}

func TestAllocatePort_FirstSlot(t *testing.T) {
	db := setupTestDB(t)
	m := NewManager(db)

	port, err := m.AllocatePort("s1", 3306, "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 3306 {
		t.Errorf("expected port 3306, got %d", port)
	}
}

func TestAllocatePort_SecondSlot(t *testing.T) {
	db := setupTestDB(t)
	m := NewManager(db)

	// Allocate first slot
	_, err := m.AllocatePort("s1", 3306, "tcp")
	if err != nil {
		t.Fatal(err)
	}

	// Allocate second slot — should be 13306
	port, err := m.AllocatePort("s3", 3306, "tcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 13306 {
		t.Errorf("expected port 13306, got %d", port)
	}
}

func TestAllocatePort_DifferentPorts(t *testing.T) {
	db := setupTestDB(t)
	m := NewManager(db)

	mysqlPort, err := m.AllocatePort("s1", 3306, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	// Should be 3306 or slightly higher if 3306 is in use on host
	if mysqlPort < 3306 || mysqlPort > 3306+100 {
		t.Errorf("expected port near 3306, got %d", mysqlPort)
	}

	redisPort, err := m.AllocatePort("s2", 6379, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	// Should be 6379 or slightly higher if 6379 is in use on host
	if redisPort < 6379 || redisPort > 6379+100 {
		t.Errorf("expected port near 6379, got %d", redisPort)
	}

	// They should be different ports
	if mysqlPort == redisPort {
		t.Error("mysql and redis should have different ports")
	}
}

func TestReleaseServicePorts(t *testing.T) {
	db := setupTestDB(t)
	m := NewManager(db)

	_, err := m.AllocatePort("s1", 3306, "tcp")
	if err != nil {
		t.Fatal(err)
	}

	allocs, err := m.GetServicePorts("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(allocs) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(allocs))
	}

	err = m.ReleaseServicePorts("s1")
	if err != nil {
		t.Fatal(err)
	}

	allocs, err = m.GetServicePorts("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(allocs) != 0 {
		t.Errorf("expected 0 allocations after release, got %d", len(allocs))
	}
}

func TestGetAllAllocations(t *testing.T) {
	db := setupTestDB(t)
	m := NewManager(db)

	m.AllocatePort("s1", 3306, "tcp")
	m.AllocatePort("s2", 6379, "tcp")

	allocs, err := m.GetAllAllocations()
	if err != nil {
		t.Fatal(err)
	}
	if len(allocs) != 2 {
		t.Errorf("expected 2 allocations, got %d", len(allocs))
	}
}
