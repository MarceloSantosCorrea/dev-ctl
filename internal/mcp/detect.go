package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type markerFile struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

// FindProjectMarker sobe no filesystem a partir de startDir procurando por um
// arquivo .devctl (igual ao que o git faz com .git). Retorna name e id do projeto.
func FindProjectMarker(startDir string) (name, id string, err error) {
	dir := startDir
	for {
		path := filepath.Join(dir, ".devctl")
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			var m markerFile
			if jsonErr := json.Unmarshal(data, &m); jsonErr == nil && m.ID != "" {
				return m.Name, m.ID, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", "", errors.New("arquivo .devctl não encontrado no diretório atual nem nos ancestrais")
}
