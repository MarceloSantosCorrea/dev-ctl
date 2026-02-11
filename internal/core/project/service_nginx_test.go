package project

import (
	"testing"
)

func TestResolveNginxDocumentRoot_ProjectWithPublicSubdir(t *testing.T) {
	svc := &Service{}
	root := svc.resolveNginxDocumentRoot(t.TempDir(), "/usr/share/nginx/html", nil)
	if root != "/usr/share/nginx/html" {
		t.Fatalf("expected /usr/share/nginx/html, got %s", root)
	}
}

func TestResolveNginxDocumentRoot_ProjectWithoutPublic(t *testing.T) {
	svc := &Service{}
	projectPath := t.TempDir()
	root := svc.resolveNginxDocumentRoot(projectPath, "/usr/share/nginx/html", nil)
	if root != "/usr/share/nginx/html" {
		t.Fatalf("expected /usr/share/nginx/html, got %s", root)
	}
}

func TestResolveNginxDocumentRoot_Override(t *testing.T) {
	svc := &Service{}
	root := svc.resolveNginxDocumentRoot(
		"/tmp/project",
		"/usr/share/nginx/html",
		map[string]interface{}{"document_root": "web"},
	)
	if root != "/usr/share/nginx/html/web" {
		t.Fatalf("expected /usr/share/nginx/html/web, got %s", root)
	}
}

func TestMapContainerRoot(t *testing.T) {
	got := mapContainerRoot("/usr/share/nginx/html/public", "/usr/share/nginx/html", "/var/www/html")
	if got != "/var/www/html/public" {
		t.Fatalf("expected /var/www/html/public, got %s", got)
	}
}
