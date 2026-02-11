import { Database, Globe, Server, MessageSquare, HardDrive, ToggleLeft, ToggleRight, Trash2, Copy, Check, Settings } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { ServiceItem } from '../lib/api'

const categoryIcons: Record<string, typeof Database> = {
  database: Database,
  web: Globe,
  cache: Server,
  runtime: Server,
  messaging: MessageSquare,
  proxy: Globe,
}

interface ServiceCardProps {
  service: ServiceItem
  onToggle?: () => void
  onDelete?: () => void
  onUpdateDocumentRoot?: (documentRoot: string) => void
  isUpdatingDocumentRoot?: boolean
}

function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = () => {
    navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <button
      onClick={handleCopy}
      className="p-0.5 rounded hover:bg-slate-200 transition-colors border-0 bg-transparent cursor-pointer"
      title="Copy"
    >
      {copied ? <Check className="w-3 h-3 text-green-500" /> : <Copy className="w-3 h-3 text-slate-400" />}
    </button>
  )
}

export default function ServiceCard({
  service,
  onToggle,
  onDelete,
  onUpdateDocumentRoot,
  isUpdatingDocumentRoot = false,
}: ServiceCardProps) {
  const Icon = categoryIcons[service.template_name] || HardDrive
  const isNginx = service.template_name === 'nginx'
  const configuredDocumentRoot =
    typeof service.config?.document_root === 'string'
      ? (service.config.document_root as string)
      : ''
  const [editingDocumentRoot, setEditingDocumentRoot] = useState(false)
  const [documentRootInput, setDocumentRootInput] = useState(configuredDocumentRoot)

  useEffect(() => {
    setDocumentRootInput(configuredDocumentRoot)
  }, [configuredDocumentRoot, service.id])

  return (
    <div className={`bg-white rounded-lg border p-4 ${service.enabled ? 'border-slate-200' : 'border-slate-100 opacity-60'}`}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg ${service.enabled ? 'bg-blue-50 text-blue-600' : 'bg-slate-100 text-slate-400'}`}>
            <Icon className="w-5 h-5" />
          </div>
          <div>
            <h4 className="font-medium text-slate-900">{service.name}</h4>
            <p className="text-sm text-slate-500">{service.template_name}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {onToggle && (
            <button
              onClick={onToggle}
              className="p-1 rounded hover:bg-slate-100 transition-colors border-0 bg-transparent cursor-pointer"
              title={service.enabled ? 'Disable' : 'Enable'}
            >
              {service.enabled ? (
                <ToggleRight className="w-5 h-5 text-blue-600" />
              ) : (
                <ToggleLeft className="w-5 h-5 text-slate-400" />
              )}
            </button>
          )}
          {onDelete && (
            <button
              onClick={onDelete}
              className="p-1 rounded hover:bg-red-50 transition-colors border-0 bg-transparent cursor-pointer"
              title="Remove service"
            >
              <Trash2 className="w-4 h-4 text-slate-400 hover:text-red-500" />
            </button>
          )}
        </div>
      </div>

      {service.ports && service.ports.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {service.ports.map((port) => (
            <span
              key={port.id}
              className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-slate-100 text-slate-700"
            >
              {port.external_port}:{port.internal_port}/{port.protocol}
            </span>
          ))}
        </div>
      )}

      {service.connection_info && Object.keys(service.connection_info).length > 0 && (
        <div className="mt-3 border-t border-slate-100 pt-3">
          <p className="text-xs font-medium text-slate-500 mb-2">Connection Info</p>
          <div className="space-y-1">
            {Object.entries(service.connection_info)
              .filter(([key]) => key !== 'connection_string')
              .map(([key, value]) => (
                <div key={key} className="flex items-center justify-between text-xs">
                  <span className="text-slate-500 capitalize">{key}</span>
                  <span className="flex items-center gap-1 font-mono text-slate-800">
                    {value}
                    <CopyButton value={value} />
                  </span>
                </div>
              ))}
            {service.connection_info.connection_string && (
              <div className="mt-2 p-2 bg-slate-50 rounded text-xs font-mono text-slate-700 flex items-center justify-between gap-2">
                <span className="truncate">{service.connection_info.connection_string}</span>
                <CopyButton value={service.connection_info.connection_string} />
              </div>
            )}
          </div>
        </div>
      )}

      {isNginx && onUpdateDocumentRoot && (
        <div className="mt-3 border-t border-slate-100 pt-3">
          <div className="flex items-center justify-between mb-2">
            <p className="text-xs font-medium text-slate-500 flex items-center gap-1">
              <Settings className="w-3.5 h-3.5" />
              Nginx Options
            </p>
            {!editingDocumentRoot && (
              <button
                onClick={() => setEditingDocumentRoot(true)}
                className="text-xs px-2 py-1 rounded bg-slate-100 text-slate-600 border-0 cursor-pointer hover:bg-slate-200"
              >
                Edit
              </button>
            )}
          </div>

          {!editingDocumentRoot ? (
            <p className="text-xs text-slate-600">
              Document root:{' '}
              <span className="font-mono text-slate-800">
                {configuredDocumentRoot || '(auto)'}
              </span>
            </p>
          ) : (
            <div className="space-y-2">
              <input
                type="text"
                value={documentRootInput}
                onChange={(e) => setDocumentRootInput(e.target.value)}
                placeholder="public"
                className="w-full px-2 py-1.5 border border-slate-300 rounded text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
              <p className="text-[11px] text-slate-500">
                Deixe vazio para usar detecção automática.
              </p>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => {
                    onUpdateDocumentRoot(documentRootInput)
                    setEditingDocumentRoot(false)
                  }}
                  disabled={isUpdatingDocumentRoot}
                  className="text-xs px-2 py-1 rounded bg-blue-600 text-white border-0 cursor-pointer hover:bg-blue-700 disabled:opacity-50"
                >
                  Save
                </button>
                <button
                  onClick={() => {
                    setDocumentRootInput(configuredDocumentRoot)
                    setEditingDocumentRoot(false)
                  }}
                  disabled={isUpdatingDocumentRoot}
                  className="text-xs px-2 py-1 rounded bg-slate-100 text-slate-700 border-0 cursor-pointer hover:bg-slate-200 disabled:opacity-50"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
