package ports

import (
	"database/sql"
	"fmt"
	"net"
	"sync"
)

const portOffset = 10000

type Manager struct {
	db *sql.DB
	mu sync.Mutex
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// AllocatePort finds the next available external port for a given internal port.
// It uses a deterministic offset strategy: slot 0 = base port, slot 1 = base + 10000, etc.
// If the calculated port is busy, it increments by 1 until a free port is found.
func (m *Manager) AllocatePort(serviceID string, internalPort int, protocol string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	slot, err := m.nextSlot(internalPort, protocol)
	if err != nil {
		return 0, err
	}

	candidatePort := internalPort + (slot * portOffset)

	// Privileged ports (< 1024) require root; skip to next slot
	if candidatePort < 1024 {
		slot = 1
		candidatePort = internalPort + portOffset
	}

	for i := 0; i < 100; i++ {
		port := candidatePort + i
		if port > 65535 {
			return 0, fmt.Errorf("no available port found for internal port %d", internalPort)
		}

		taken, err := m.isPortAllocated(port, protocol)
		if err != nil {
			return 0, err
		}
		if taken {
			continue
		}

		if !isPortAvailable(port, protocol) {
			continue
		}

		if err := m.saveAllocation(serviceID, internalPort, port, protocol); err != nil {
			return 0, err
		}

		return port, nil
	}

	return 0, fmt.Errorf("no available port found after 100 attempts for internal port %d", internalPort)
}

// ReleaseServicePorts removes all port allocations for a given service.
func (m *Manager) ReleaseServicePorts(serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec("DELETE FROM port_allocations WHERE service_id = ?", serviceID)
	return err
}

// GetServicePorts returns all port allocations for a service.
func (m *Manager) GetServicePorts(serviceID string) ([]PortAllocation, error) {
	rows, err := m.db.Query(
		"SELECT id, service_id, internal_port, external_port, protocol FROM port_allocations WHERE service_id = ?",
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []PortAllocation
	for rows.Next() {
		var a PortAllocation
		if err := rows.Scan(&a.ID, &a.ServiceID, &a.InternalPort, &a.ExternalPort, &a.Protocol); err != nil {
			return nil, err
		}
		allocs = append(allocs, a)
	}
	return allocs, rows.Err()
}

// GetAllAllocations returns all port allocations in the database.
func (m *Manager) GetAllAllocations() ([]PortAllocation, error) {
	rows, err := m.db.Query(
		"SELECT id, service_id, internal_port, external_port, protocol FROM port_allocations",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []PortAllocation
	for rows.Next() {
		var a PortAllocation
		if err := rows.Scan(&a.ID, &a.ServiceID, &a.InternalPort, &a.ExternalPort, &a.Protocol); err != nil {
			return nil, err
		}
		allocs = append(allocs, a)
	}
	return allocs, rows.Err()
}

func (m *Manager) nextSlot(internalPort int, protocol string) (int, error) {
	var maxExternal sql.NullInt64
	err := m.db.QueryRow(
		"SELECT MAX(external_port) FROM port_allocations WHERE internal_port = ? AND protocol = ?",
		internalPort, protocol,
	).Scan(&maxExternal)
	if err != nil {
		return 0, err
	}

	if !maxExternal.Valid {
		return 0, nil
	}

	slot := (int(maxExternal.Int64) - internalPort) / portOffset
	return slot + 1, nil
}

func (m *Manager) isPortAllocated(port int, protocol string) (bool, error) {
	var count int
	err := m.db.QueryRow(
		"SELECT COUNT(*) FROM port_allocations WHERE external_port = ? AND protocol = ?",
		port, protocol,
	).Scan(&count)
	return count > 0, err
}

func (m *Manager) saveAllocation(serviceID string, internalPort, externalPort int, protocol string) error {
	id := fmt.Sprintf("pa_%d_%s", externalPort, protocol)
	_, err := m.db.Exec(
		"INSERT INTO port_allocations (id, service_id, internal_port, external_port, protocol) VALUES (?, ?, ?, ?, ?)",
		id, serviceID, internalPort, externalPort, protocol,
	)
	return err
}

// browserUnsafePorts lists ports that Chromium-based browsers refuse to connect to.
// https://chromium.googlesource.com/chromium/src/+/refs/heads/main/net/base/port_util.cc
var browserUnsafePorts = map[int]bool{
	10080: true, // Amanda
	6566:  true, // SANE
	6665:  true, 6666: true, 6667: true, 6668: true, 6669: true, // IRC
	6697: true, // IRC+TLS
}

func isPortAvailable(port int, protocol string) bool {
	if browserUnsafePorts[port] {
		return false
	}
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen(protocol, addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

type PortAllocation struct {
	ID           string `json:"id"`
	ServiceID    string `json:"service_id"`
	InternalPort int    `json:"internal_port"`
	ExternalPort int    `json:"external_port"`
	Protocol     string `json:"protocol"`
}
