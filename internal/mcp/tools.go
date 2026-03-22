package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/marcelo/devctl/internal/core/project"
)

type handlers struct {
	svc         *project.Service
	templateDir string
}

func (h *handlers) listProjects(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projects, err := h.svc.ListAllProjects(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("erro ao listar projetos: %v", err)), nil
	}

	b, _ := json.MarshalIndent(projects, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (h *handlers) projectStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, _ := req.GetArguments()["name"].(string)

	if name == "" {
		cwd, _ := os.Getwd()
		n, _, err := FindProjectMarker(cwd)
		if err != nil {
			return mcp.NewToolResultError("parâmetro 'name' não fornecido e nenhum .devctl encontrado no diretório atual"), nil
		}
		name = n
	}

	projects, err := h.svc.ListAllProjects(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("erro ao listar projetos: %v", err)), nil
	}

	for _, p := range projects {
		if p.Name == name {
			b, _ := json.MarshalIndent(p, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("projeto %q não encontrado", name)), nil
}

func (h *handlers) projectUp(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, name, err := h.resolveProjectID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := h.svc.ProjectUpByID(ctx, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("erro ao subir projeto %q: %v", name, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Projeto %q iniciado com sucesso.", name)), nil
}

func (h *handlers) projectDown(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, name, err := h.resolveProjectID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := h.svc.ProjectDownByID(ctx, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("erro ao parar projeto %q: %v", name, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Projeto %q parado com sucesso.", name)), nil
}

func (h *handlers) projectRebuild(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, name, err := h.resolveProjectID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := h.svc.ProjectRebuildByID(ctx, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("erro ao rebuild do projeto %q: %v", name, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Projeto %q reconstruído com sucesso.", name)), nil
}

func (h *handlers) listTemplates(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templates, err := project.ListTemplates(h.templateDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("erro ao listar templates: %v", err)), nil
	}

	b, _ := json.MarshalIndent(templates, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

// resolveProjectID obtém o ID do projeto a partir do parâmetro 'name' ou do .devctl do diretório atual.
func (h *handlers) resolveProjectID(ctx context.Context, req mcp.CallToolRequest) (id, name string, err error) {
	name, _ = req.GetArguments()["name"].(string)

	if name == "" {
		cwd, _ := os.Getwd()
		n, i, markerErr := FindProjectMarker(cwd)
		if markerErr != nil {
			return "", "", fmt.Errorf("parâmetro 'name' não fornecido e nenhum .devctl encontrado no diretório atual")
		}
		return i, n, nil
	}

	projects, err := h.svc.ListAllProjects(ctx)
	if err != nil {
		return "", "", fmt.Errorf("erro ao listar projetos: %v", err)
	}

	for _, p := range projects {
		if p.Name == name {
			return p.ID, p.Name, nil
		}
	}

	return "", "", fmt.Errorf("projeto %q não encontrado", name)
}

func registerTools(s *server.MCPServer, h *handlers) {
	s.AddTool(mcp.NewTool("list_projects",
		mcp.WithDescription("Lista todos os projetos gerenciados pelo devctl com seus status"),
	), h.listProjects)

	s.AddTool(mcp.NewTool("project_status",
		mcp.WithDescription("Retorna o status detalhado de um projeto. Se 'name' não for fornecido, auto-detecta pelo .devctl do diretório atual"),
		mcp.WithString("name",
			mcp.Description("Nome do projeto (opcional — auto-detectado pelo .devctl se omitido)"),
		),
	), h.projectStatus)

	s.AddTool(mcp.NewTool("project_up",
		mcp.WithDescription("Inicia os containers de um projeto. Se 'name' não for fornecido, auto-detecta pelo .devctl do diretório atual"),
		mcp.WithString("name",
			mcp.Description("Nome do projeto (opcional — auto-detectado pelo .devctl se omitido)"),
		),
	), h.projectUp)

	s.AddTool(mcp.NewTool("project_down",
		mcp.WithDescription("Para os containers de um projeto. Se 'name' não for fornecido, auto-detecta pelo .devctl do diretório atual"),
		mcp.WithString("name",
			mcp.Description("Nome do projeto (opcional — auto-detectado pelo .devctl se omitido)"),
		),
	), h.projectDown)

	s.AddTool(mcp.NewTool("project_rebuild",
		mcp.WithDescription("Para, reconstrói e reinicia os containers de um projeto. Se 'name' não for fornecido, auto-detecta pelo .devctl do diretório atual"),
		mcp.WithString("name",
			mcp.Description("Nome do projeto (opcional — auto-detectado pelo .devctl se omitido)"),
		),
	), h.projectRebuild)

	s.AddTool(mcp.NewTool("list_templates",
		mcp.WithDescription("Lista todos os templates de serviço disponíveis no devctl"),
	), h.listTemplates)
}
