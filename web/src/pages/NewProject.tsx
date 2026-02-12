import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Check, ChevronRight, Loader2, Plus, X } from 'lucide-react'
import { api, type Template } from '../lib/api'
import type { ExtraPort } from '../components/ServiceCard'

type Step = 'name' | 'services' | 'review'

interface SelectedService {
  template_name: string
  name: string
  config: Record<string, unknown>
}

export default function NewProject() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [step, setStep] = useState<Step>('name')
  const [projectName, setProjectName] = useState('')
  const [projectPath, setProjectPath] = useState('')
  const [selectedServices, setSelectedServices] = useState<SelectedService[]>([])
  const [sslEnabled, setSSLEnabled] = useState(false)

  const { data: templates } = useQuery({
    queryKey: ['templates'],
    queryFn: api.listTemplates,
  })

  const createMutation = useMutation({
    mutationFn: () =>
      api.createProject({
        name: projectName,
        path: projectPath || undefined,
        ssl_enabled: sslEnabled,
        services: selectedServices,
      }),
    onSuccess: (project) => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      navigate(`/projects/${project.id}`)
    },
  })

  const addService = (template: Template) => {
    const defaultConfig: Record<string, unknown> = {}
    if (template.environment) {
      const envDefaults: Record<string, string> = {}
      for (const [key, val] of Object.entries(template.environment)) {
        envDefaults[key] = val.default
      }
      defaultConfig.environment = envDefaults
    }

    setSelectedServices([
      ...selectedServices,
      {
        template_name: template.name,
        name: template.name,
        config: defaultConfig,
      },
    ])
  }

  const removeService = (index: number) => {
    setSelectedServices(selectedServices.filter((_, i) => i !== index))
  }

  const updateServiceConfig = (
    templateName: string,
    updater: (currentConfig: Record<string, unknown>) => Record<string, unknown>
  ) => {
    setSelectedServices((prev) =>
      prev.map((svc) =>
        svc.template_name === templateName
          ? { ...svc, config: updater(svc.config || {}) }
          : svc
      )
    )
  }

  const nginxService = selectedServices.find((s) => s.template_name === 'nginx')

  // Reset sslEnabled if nginx is removed
  useEffect(() => {
    if (!nginxService) {
      setSSLEnabled(false)
    }
  }, [nginxService])
  const nginxDocumentRoot =
    typeof nginxService?.config?.document_root === 'string'
      ? (nginxService.config.document_root as string)
      : ''

  const isServiceSelected = (name: string) =>
    selectedServices.some((s) => s.template_name === name)

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold text-slate-900 mb-6">Novo Projeto</h1>

      {/* Steps indicator */}
      <div className="flex items-center gap-2 mb-8">
        {(['name', 'services', 'review'] as Step[]).map((s, i) => (
          <div key={s} className="flex items-center gap-2">
            {i > 0 && <ChevronRight className="w-4 h-4 text-slate-300" />}
            <button
              onClick={() => {
                if (s === 'name' || (s === 'services' && projectName) || (s === 'review' && projectName && selectedServices.length > 0)) {
                  setStep(s)
                }
              }}
              className={`px-3 py-1 rounded-full text-sm font-medium border-0 cursor-pointer transition-colors ${
                step === s
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
              }`}
            >
              {s === 'name' ? '1. Nome' : s === 'services' ? '2. Serviços' : '3. Revisão'}
            </button>
          </div>
        ))}
      </div>

      {/* Step: Name */}
      {step === 'name' && (
        <div className="bg-white rounded-lg border border-slate-200 p-6">
          <h2 className="text-lg font-medium text-slate-900 mb-4">Nome do Projeto</h2>
          <input
            type="text"
            value={projectName}
            onChange={(e) => setProjectName(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
            placeholder="meu-projeto"
            className="w-full px-4 py-2 border border-slate-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
          <p className="mt-2 text-sm text-slate-500">
            Domínio: <span className="font-medium">{projectName || '...'}.local</span>
          </p>

          <h2 className="text-lg font-medium text-slate-900 mt-6 mb-4">Caminho do Projeto</h2>
          <input
            type="text"
            value={projectPath}
            onChange={(e) => setProjectPath(e.target.value)}
            placeholder="/home/marcelo/developer/meu-projeto"
            className="w-full px-4 py-2 border border-slate-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
          <p className="mt-2 text-sm text-slate-500">
            Diretório local com seu código-fonte. Será montado nos containers web/runtime.
          </p>
          <button
            onClick={() => setStep('services')}
            disabled={!projectName}
            className="mt-4 px-4 py-2 bg-blue-600 text-white rounded-md text-sm font-medium border-0 cursor-pointer hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Próximo
          </button>
        </div>
      )}

      {/* Step: Services */}
      {step === 'services' && (
        <div className="bg-white rounded-lg border border-slate-200 p-6">
          <h2 className="text-lg font-medium text-slate-900 mb-4">Selecionar Serviços</h2>

          {selectedServices.length > 0 && (
            <div className="mb-4">
              <h3 className="text-sm font-medium text-slate-700 mb-2">Selecionados:</h3>
              <div className="flex flex-wrap gap-2">
                {selectedServices.map((svc, i) => (
                  <span
                    key={i}
                    className="inline-flex items-center gap-1 px-3 py-1 bg-blue-50 text-blue-700 rounded-full text-sm"
                  >
                    {svc.template_name}
                    <button
                      onClick={() => removeService(i)}
                      className="p-0 bg-transparent border-0 cursor-pointer"
                    >
                      <X className="w-3.5 h-3.5 text-blue-400 hover:text-blue-600" />
                    </button>
                  </span>
                ))}
              </div>
            </div>
          )}

          {nginxService && (
            <div className="mb-4 p-4 rounded-lg border border-slate-200 bg-slate-50">
              <h3 className="text-sm font-medium text-slate-800 mb-2">Opções do Nginx</h3>
              <label className="block text-sm text-slate-700 mb-1">Document Root (opcional)</label>
              <input
                type="text"
                value={nginxDocumentRoot}
                onChange={(e) => {
                  const value = e.target.value.trim()
                  updateServiceConfig('nginx', (currentConfig) => {
                    const nextConfig = { ...currentConfig }
                    if (value) {
                      nextConfig.document_root = value
                    } else {
                      delete nextConfig.document_root
                    }
                    return nextConfig
                  })
                }}
                placeholder="public"
                className="w-full px-3 py-2 border border-slate-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
              <p className="mt-2 text-xs text-slate-500">
                Relative value (ex.: <span className="font-mono">public</span>) usa o diretório montado no
                container. Caminho absoluto também é aceito.
              </p>
              <label className="flex items-center gap-2 mt-3">
                <input
                  type="checkbox"
                  checked={sslEnabled}
                  onChange={(e) => setSSLEnabled(e.target.checked)}
                  className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                />
                <span className="text-sm text-slate-700">Habilitar SSL (HTTPS)</span>
              </label>
            </div>
          )}

          {selectedServices.map((svc) => {
            const tmpl = templates?.find((t) => t.name === svc.template_name)
            if (!tmpl?.versions?.length) return null
            const currentImage = (svc.config.image as string) || tmpl.default_image
            return (
              <div key={svc.template_name} className="mb-4 p-4 rounded-lg border border-slate-200 bg-slate-50">
                <h3 className="text-sm font-medium text-slate-800 mb-2">Versão do {tmpl.display_name}</h3>
                <select
                  value={currentImage}
                  onChange={(e) => {
                    const value = e.target.value
                    updateServiceConfig(svc.template_name, (currentConfig) => {
                      const nextConfig = { ...currentConfig }
                      if (value === tmpl.default_image) {
                        delete nextConfig.image
                      } else {
                        nextConfig.image = value
                      }
                      return nextConfig
                    })
                  }}
                  className="w-full px-3 py-2 border border-slate-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white"
                >
                  {tmpl.versions.map((v) => (
                    <option key={v.image} value={v.image}>
                      {v.label}{v.image === tmpl.default_image ? ' (padrão)' : ''}
                    </option>
                  ))}
                </select>
              </div>
            )
          })}

          {selectedServices.map((svc) => {
            const extraPorts: ExtraPort[] = Array.isArray(svc.config?.extra_ports)
              ? (svc.config.extra_ports as ExtraPort[])
              : []
            const tmpl = templates?.find((t) => t.name === svc.template_name)
            return (
              <div key={`extra-${svc.template_name}`} className="mb-4 p-4 rounded-lg border border-slate-200 bg-slate-50">
                <h3 className="text-sm font-medium text-slate-800 mb-2">
                  Portas Extras — {tmpl?.display_name || svc.template_name}
                </h3>
                {extraPorts.map((ep, i) => (
                  <div key={i} className="flex items-center gap-2 mb-2">
                    <input
                      type="number"
                      min={1}
                      max={65535}
                      value={ep.internal || ''}
                      onChange={(e) => {
                        const updated = [...extraPorts]
                        updated[i] = { ...updated[i], internal: parseInt(e.target.value) || 0 }
                        updateServiceConfig(svc.template_name, (cfg) => ({ ...cfg, extra_ports: updated }))
                      }}
                      placeholder="Porta"
                      className="w-24 px-2 py-1.5 border border-slate-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                    <select
                      value={ep.protocol}
                      onChange={(e) => {
                        const updated = [...extraPorts]
                        updated[i] = { ...updated[i], protocol: e.target.value }
                        updateServiceConfig(svc.template_name, (cfg) => ({ ...cfg, extra_ports: updated }))
                      }}
                      className="px-2 py-1.5 border border-slate-300 rounded-md text-sm bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="tcp">tcp</option>
                      <option value="udp">udp</option>
                    </select>
                    <input
                      type="text"
                      value={ep.description}
                      onChange={(e) => {
                        const updated = [...extraPorts]
                        updated[i] = { ...updated[i], description: e.target.value }
                        updateServiceConfig(svc.template_name, (cfg) => ({ ...cfg, extra_ports: updated }))
                      }}
                      placeholder="Descrição"
                      className="flex-1 px-2 py-1.5 border border-slate-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                    <button
                      onClick={() => {
                        const updated = extraPorts.filter((_, j) => j !== i)
                        updateServiceConfig(svc.template_name, (cfg) => {
                          const next = { ...cfg }
                          if (updated.length > 0) {
                            next.extra_ports = updated
                          } else {
                            delete next.extra_ports
                          }
                          return next
                        })
                      }}
                      className="p-1 bg-transparent border-0 cursor-pointer rounded hover:bg-red-50"
                    >
                      <X className="w-4 h-4 text-slate-400 hover:text-red-500" />
                    </button>
                  </div>
                ))}
                <button
                  onClick={() => {
                    const updated = [...extraPorts, { internal: 0, protocol: 'tcp', description: '' }]
                    updateServiceConfig(svc.template_name, (cfg) => ({ ...cfg, extra_ports: updated }))
                  }}
                  className="flex items-center gap-1 text-sm px-3 py-1.5 rounded-md bg-white text-slate-600 border border-dashed border-slate-300 cursor-pointer hover:bg-slate-100"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Adicionar porta
                </button>
              </div>
            )
          })}

          <div className="grid grid-cols-2 gap-3">
            {templates?.map((tmpl) => (
              <button
                key={tmpl.name}
                onClick={() => !isServiceSelected(tmpl.name) && addService(tmpl)}
                disabled={isServiceSelected(tmpl.name)}
                className={`text-left p-4 rounded-lg border-2 transition-colors cursor-pointer ${
                  isServiceSelected(tmpl.name)
                    ? 'border-blue-300 bg-blue-50'
                    : 'border-slate-200 bg-white hover:border-blue-300'
                }`}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-slate-900">{tmpl.display_name}</span>
                  {isServiceSelected(tmpl.name) && <Check className="w-4 h-4 text-blue-600" />}
                </div>
                <p className="text-sm text-slate-500 mt-1">{tmpl.description}</p>
                <span className="inline-block mt-2 px-2 py-0.5 bg-slate-100 rounded text-xs text-slate-600">
                  {tmpl.category}
                </span>
              </button>
            )) || (
              <p className="text-slate-500 col-span-2">Nenhum template disponível</p>
            )}
          </div>

          <div className="flex items-center gap-3 mt-6">
            <button
              onClick={() => setStep('name')}
              className="px-4 py-2 bg-slate-100 text-slate-700 rounded-md text-sm font-medium border-0 cursor-pointer hover:bg-slate-200"
            >
              Voltar
            </button>
            <button
              onClick={() => setStep('review')}
              disabled={selectedServices.length === 0}
              className="px-4 py-2 bg-blue-600 text-white rounded-md text-sm font-medium border-0 cursor-pointer hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Próximo
            </button>
          </div>
        </div>
      )}

      {/* Step: Review */}
      {step === 'review' && (
        <div className="bg-white rounded-lg border border-slate-200 p-6">
          <h2 className="text-lg font-medium text-slate-900 mb-4">Revisão</h2>

          <dl className="divide-y divide-slate-100">
            <div className="py-3 flex justify-between">
              <dt className="text-sm font-medium text-slate-500">Nome do Projeto</dt>
              <dd className="text-sm text-slate-900">{projectName}</dd>
            </div>
            <div className="py-3 flex justify-between">
              <dt className="text-sm font-medium text-slate-500">Domínio</dt>
              <dd className="text-sm text-blue-600">{projectName}.local</dd>
            </div>
            {nginxService && (
              <div className="py-3 flex justify-between">
                <dt className="text-sm font-medium text-slate-500">SSL</dt>
                <dd className={`text-sm font-medium ${sslEnabled ? 'text-green-600' : 'text-slate-500'}`}>
                  {sslEnabled ? 'Habilitado' : 'Desabilitado'}
                </dd>
              </div>
            )}
            {projectPath && (
              <div className="py-3 flex justify-between">
                <dt className="text-sm font-medium text-slate-500">Caminho</dt>
                <dd className="text-sm text-slate-900 font-mono">{projectPath}</dd>
              </div>
            )}
            <div className="py-3">
              <dt className="text-sm font-medium text-slate-500 mb-2">Serviços</dt>
              <dd>
                <div className="flex flex-wrap gap-2">
                  {selectedServices.map((svc, i) => (
                    <span
                      key={i}
                      className="px-3 py-1 bg-slate-100 text-slate-700 rounded-full text-sm"
                    >
                      {svc.template_name}
                    </span>
                  ))}
                </div>
              </dd>
            </div>
            {nginxDocumentRoot && (
              <div className="py-3 flex justify-between">
                <dt className="text-sm font-medium text-slate-500">Nginx Document Root</dt>
                <dd className="text-sm text-slate-900 font-mono">{nginxDocumentRoot}</dd>
              </div>
            )}
            {selectedServices.map((svc) => {
              const tmpl = templates?.find((t) => t.name === svc.template_name)
              const customImage = svc.config.image as string | undefined
              if (!customImage || !tmpl) return null
              const version = tmpl.versions?.find((v) => v.image === customImage)
              return (
                <div key={svc.template_name} className="py-3 flex justify-between">
                  <dt className="text-sm font-medium text-slate-500">Versão {tmpl.display_name}</dt>
                  <dd className="text-sm text-slate-900">{version?.label || customImage}</dd>
                </div>
              )
            })}
            {selectedServices.map((svc) => {
              const extraPorts = Array.isArray(svc.config?.extra_ports) ? (svc.config.extra_ports as ExtraPort[]) : []
              if (extraPorts.length === 0) return null
              const tmpl = templates?.find((t) => t.name === svc.template_name)
              return (
                <div key={`review-ep-${svc.template_name}`} className="py-3">
                  <dt className="text-sm font-medium text-slate-500 mb-1">
                    Portas Extras — {tmpl?.display_name || svc.template_name}
                  </dt>
                  <dd className="flex flex-wrap gap-1">
                    {extraPorts.map((ep, i) => (
                      <span key={i} className="px-2 py-0.5 bg-amber-50 text-amber-700 rounded text-xs font-medium">
                        {ep.internal}/{ep.protocol}{ep.description ? ` - ${ep.description}` : ''}
                      </span>
                    ))}
                  </dd>
                </div>
              )
            })}
          </dl>

          {createMutation.error && (
            <div className="mt-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">
              {(createMutation.error as Error).message}
            </div>
          )}

          <div className="flex items-center gap-3 mt-6">
            <button
              onClick={() => setStep('services')}
              className="px-4 py-2 bg-slate-100 text-slate-700 rounded-md text-sm font-medium border-0 cursor-pointer hover:bg-slate-200"
            >
              Voltar
            </button>
            <button
              onClick={() => createMutation.mutate()}
              disabled={createMutation.isPending}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md text-sm font-medium border-0 cursor-pointer hover:bg-blue-700 disabled:opacity-50"
            >
              {createMutation.isPending ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Plus className="w-4 h-4" />
              )}
              Criar Projeto
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
