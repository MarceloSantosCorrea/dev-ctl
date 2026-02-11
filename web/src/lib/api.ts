const BASE_URL = '/api';

export interface Project {
  id: string;
  name: string;
  domain: string;
  path: string;
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
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || 'Request failed');
  }
  return res.json();
}

export const api = {
  // Projects
  listProjects: () => request<Project[]>('/projects'),
  getProject: (id: string) => request<Project>(`/projects/${id}`),
  createProject: (data: { name: string; path?: string; services: { template_name: string; name: string; config?: Record<string, unknown> }[] }) =>
    request<Project>('/projects', { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (id: string, data: { name: string }) =>
    request<Project>(`/projects/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProject: (id: string) =>
    request<{ status: string }>(`/projects/${id}`, { method: 'DELETE' }),
  projectUp: (id: string) =>
    request<{ status: string }>(`/projects/${id}/up`, { method: 'POST' }),
  projectDown: (id: string) =>
    request<{ status: string }>(`/projects/${id}/down`, { method: 'POST' }),

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
