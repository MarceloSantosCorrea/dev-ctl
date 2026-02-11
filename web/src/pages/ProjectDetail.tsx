import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Play, Square, Trash2, ExternalLink, Loader2, AlertTriangle, ArrowLeft, CircleDot, FolderOpen,
} from 'lucide-react'
import { api } from '../lib/api'
import ServiceCard from '../components/ServiceCard'
import PortTable from '../components/PortTable'
import LogViewer from '../components/LogViewer'

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: project, isLoading, error } = useQuery({
    queryKey: ['project', id],
    queryFn: () => api.getProject(id!),
    enabled: !!id,
    refetchInterval: 5000,
  })

  const upMutation = useMutation({
    mutationFn: () => api.projectUp(id!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['project', id] }),
  })

  const downMutation = useMutation({
    mutationFn: () => api.projectDown(id!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['project', id] }),
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteProject(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      navigate('/')
    },
  })

  const toggleMutation = useMutation({
    mutationFn: (serviceId: string) => api.toggleService(id!, serviceId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['project', id] }),
  })

  const deleteServiceMutation = useMutation({
    mutationFn: (serviceId: string) => api.deleteService(id!, serviceId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['project', id] }),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="w-6 h-6 animate-spin text-slate-400" />
      </div>
    )
  }

  if (error || !project) {
    return (
      <div className="bg-red-50 text-red-700 rounded-lg p-4 flex items-center gap-2">
        <AlertTriangle className="w-5 h-5" />
        Project not found.
      </div>
    )
  }

  const statusColors = {
    running: 'text-green-600 bg-green-50',
    stopped: 'text-slate-500 bg-slate-100',
    error: 'text-red-600 bg-red-50',
  }

  const allPorts = project.services?.flatMap((s) => s.ports || []) || []

  return (
    <div>
      {/* Back link */}
      <button
        onClick={() => navigate('/')}
        className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 bg-transparent border-0 cursor-pointer mb-4"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to Dashboard
      </button>

      {/* Project Header */}
      <div className="bg-white rounded-lg border border-slate-200 p-6 mb-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-2xl font-bold text-slate-900">{project.name}</h1>
              <span
                className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium ${statusColors[project.status]}`}
              >
                <CircleDot className="w-3 h-3" />
                {project.status}
              </span>
            </div>
            <a
              href={`https://${project.domain}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-blue-600 no-underline hover:underline flex items-center gap-1"
            >
              https://{project.domain}
              <ExternalLink className="w-3 h-3" />
            </a>
            {project.path && (
              <p className="text-sm text-slate-500 flex items-center gap-1 mt-1">
                <FolderOpen className="w-3 h-3" />
                <span className="font-mono">{project.path}</span>
              </p>
            )}
          </div>

          <div className="flex items-center gap-2">
            {project.status === 'running' ? (
              <button
                onClick={() => downMutation.mutate()}
                disabled={downMutation.isPending}
                className="flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium bg-slate-100 text-slate-700 hover:bg-slate-200 border-0 cursor-pointer disabled:opacity-50"
              >
                {downMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Square className="w-4 h-4" />}
                Stop
              </button>
            ) : (
              <button
                onClick={() => upMutation.mutate()}
                disabled={upMutation.isPending}
                className="flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium bg-green-50 text-green-700 hover:bg-green-100 border-0 cursor-pointer disabled:opacity-50"
              >
                {upMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                Start
              </button>
            )}
            <button
              onClick={() => {
                if (window.confirm(`Delete project "${project.name}"? This cannot be undone.`)) {
                  deleteMutation.mutate()
                }
              }}
              disabled={deleteMutation.isPending}
              className="flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium bg-red-50 text-red-600 hover:bg-red-100 border-0 cursor-pointer disabled:opacity-50"
            >
              <Trash2 className="w-4 h-4" />
              Delete
            </button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Services */}
        <div className="lg:col-span-2">
          <h2 className="text-lg font-semibold text-slate-900 mb-4">
            Services ({project.services?.length || 0})
          </h2>
          <div className="space-y-3">
            {project.services?.map((svc) => (
              <ServiceCard
                key={svc.id}
                service={svc}
                onToggle={() => toggleMutation.mutate(svc.id)}
                onDelete={() => {
                  if (window.confirm(`Remove service "${svc.name}"?`)) {
                    deleteServiceMutation.mutate(svc.id)
                  }
                }}
              />
            ))}
            {(!project.services || project.services.length === 0) && (
              <p className="text-slate-500 text-sm">No services configured.</p>
            )}
          </div>
        </div>

        {/* Ports sidebar */}
        <div>
          <h2 className="text-lg font-semibold text-slate-900 mb-4">Port Allocations</h2>
          <div className="bg-white rounded-lg border border-slate-200 p-4">
            <PortTable ports={allPorts} />
          </div>
        </div>
      </div>

      {/* Logs */}
      <div className="mt-6">
        <h2 className="text-lg font-semibold text-slate-900 mb-4">Logs</h2>
        <LogViewer projectId={id!} />
      </div>
    </div>
  )
}
