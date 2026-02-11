import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Play, Square, ExternalLink, Globe, CircleDot, AlertTriangle, Loader2 } from 'lucide-react'
import { api, type Project } from '../lib/api'

export default function Dashboard() {
  const queryClient = useQueryClient()
  const { data: projects, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: api.listProjects,
    refetchInterval: 5000,
  })

  const upMutation = useMutation({
    mutationFn: (id: string) => api.projectUp(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
  })

  const downMutation = useMutation({
    mutationFn: (id: string) => api.projectDown(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="w-6 h-6 animate-spin text-slate-400" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="bg-red-50 text-red-700 rounded-lg p-4 flex items-center gap-2">
        <AlertTriangle className="w-5 h-5" />
        Failed to load projects. Make sure the devctl server is running.
      </div>
    )
  }

  if (!projects || projects.length === 0) {
    return (
      <div className="text-center py-20">
        <Globe className="w-12 h-12 mx-auto text-slate-300 mb-4" />
        <h2 className="text-lg font-medium text-slate-900 mb-2">No projects yet</h2>
        <p className="text-slate-500 mb-6">Create your first project to get started.</p>
        <Link
          to="/new"
          className="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-md text-sm font-medium no-underline hover:bg-blue-700"
        >
          Create Project
        </Link>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-2xl font-bold text-slate-900 mb-6">Projects</h1>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {projects.map((project) => (
          <ProjectCard
            key={project.id}
            project={project}
            onUp={() => upMutation.mutate(project.id)}
            onDown={() => downMutation.mutate(project.id)}
            isActioning={upMutation.isPending || downMutation.isPending}
          />
        ))}
      </div>
    </div>
  )
}

function ProjectCard({
  project,
  onUp,
  onDown,
  isActioning,
}: {
  project: Project
  onUp: () => void
  onDown: () => void
  isActioning: boolean
}) {
  const statusColors = {
    running: 'text-green-600 bg-green-50',
    stopped: 'text-slate-500 bg-slate-100',
    error: 'text-red-600 bg-red-50',
  }

  return (
    <div className="bg-white rounded-lg border border-slate-200 p-5 hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between mb-3">
        <div>
          <Link
            to={`/projects/${project.id}`}
            className="text-lg font-semibold text-slate-900 no-underline hover:text-blue-600"
          >
            {project.name}
          </Link>
          <div className="flex items-center gap-1.5 mt-1">
            <a
              href={`https://${project.domain}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-blue-600 no-underline hover:underline flex items-center gap-1"
            >
              {project.domain}
              <ExternalLink className="w-3 h-3" />
            </a>
          </div>
        </div>
        <span
          className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium ${statusColors[project.status]}`}
        >
          <CircleDot className="w-3 h-3" />
          {project.status}
        </span>
      </div>

      <div className="text-sm text-slate-500 mb-4">
        {project.services?.length || 0} service{(project.services?.length || 0) !== 1 ? 's' : ''}
      </div>

      <div className="flex items-center gap-2">
        {project.status === 'running' ? (
          <button
            onClick={onDown}
            disabled={isActioning}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-slate-100 text-slate-700 hover:bg-slate-200 border-0 cursor-pointer transition-colors disabled:opacity-50"
          >
            <Square className="w-3.5 h-3.5" />
            Stop
          </button>
        ) : (
          <button
            onClick={onUp}
            disabled={isActioning}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-green-50 text-green-700 hover:bg-green-100 border-0 cursor-pointer transition-colors disabled:opacity-50"
          >
            <Play className="w-3.5 h-3.5" />
            Start
          </button>
        )}
        <Link
          to={`/projects/${project.id}`}
          className="px-3 py-1.5 rounded-md text-sm font-medium text-slate-600 hover:bg-slate-100 no-underline transition-colors"
        >
          Details
        </Link>
      </div>
    </div>
  )
}
