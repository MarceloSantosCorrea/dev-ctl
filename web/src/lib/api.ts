const BASE_URL = '/api';

export interface User {
  id: string;
  name: string;
  email: string;
  created_at: string;
}

export interface Project {
  id: string;
  name: string;
  domain: string;
  path: string;
  ssl_enabled: boolean;
  status: 'stopped' | 'running' | 'error';
  created_at: string;
  updated_at: string;
  services: ServiceItem[];
}

export interface ServiceItem {
  id: string;
  project_id: string;
  template_name: string;
  name: string;
  enabled: boolean;
  config: Record<string, unknown>;
  created_at: string;
  ports: PortAllocation[];
  connection_info?: Record<string, string>;
}

export interface PortAllocation {
  id: string;
  service_id: string;
  internal_port: number;
  external_port: number;
  protocol: string;
}

export interface Template {
  name: string;
  display_name: string;
  category: string;
  description: string;
  default_image: string;
  versions?: { label: string; image: string }[];
  ports: { internal: number; protocol: string; description: string }[];
  environment: Record<string, { default: string; description: string }>;
  volumes: { target: string; description: string }[];
}

export interface SystemStatus {
  docker: boolean;
  version: string;
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    ...options,
  });
  if (!res.ok) {
    if (res.status === 401 && !url.startsWith('/auth/')) {
      window.location.href = '/login';
      throw new Error('Unauthorized');
    }
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || 'Request failed');
  }
  return res.json();
}

export const api = {
  // Auth
  register: (data: { name: string; email: string; password: string }) =>
    request<User>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
  login: (data: { email: string; password: string }) =>
    request<User>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  logout: () =>
    request<{ status: string }>('/auth/logout', { method: 'POST' }),
  me: () => request<User>('/auth/me'),

  // Projects
  listProjects: () => request<Project[]>('/projects'),
  getProject: (id: string) => request<Project>(`/projects/${id}`),
  createProject: (data: { name: string; path?: string; ssl_enabled?: boolean; services: { template_name: string; name: string; config?: Record<string, unknown> }[] }) =>
    request<Project>('/projects', { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (id: string, data: { name?: string; path?: string; ssl_enabled?: boolean }) =>
    request<Project>(`/projects/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProject: (id: string) =>
    request<{ status: string }>(`/projects/${id}`, { method: 'DELETE' }),
  projectUp: (id: string) =>
    request<{ status: string }>(`/projects/${id}/up`, { method: 'POST' }),
  projectDown: (id: string) =>
    request<{ status: string }>(`/projects/${id}/down`, { method: 'POST' }),
  projectRebuild: (id: string) =>
    request<{ status: string }>(`/projects/${id}/rebuild`, { method: 'POST' }),

  // Services
  addService: (projectId: string, data: { template_name: string; name: string; config?: Record<string, unknown> }) =>
    request<ServiceItem>(`/projects/${projectId}/services`, { method: 'POST', body: JSON.stringify(data) }),
  updateService: (projectId: string, serviceId: string, data: { name: string; config: Record<string, unknown> }) =>
    request<ServiceItem>(`/projects/${projectId}/services/${serviceId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteService: (projectId: string, serviceId: string) =>
    request<{ status: string }>(`/projects/${projectId}/services/${serviceId}`, { method: 'DELETE' }),
  toggleService: (projectId: string, serviceId: string) =>
    request<ServiceItem>(`/projects/${projectId}/services/${serviceId}/toggle`, { method: 'PATCH' }),

  // Templates
  listTemplates: () => request<Template[]>('/templates'),

  // System
  systemStatus: () => request<SystemStatus>('/system/status'),
};
