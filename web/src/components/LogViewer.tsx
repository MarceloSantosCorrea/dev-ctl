import { useEffect, useRef, useState } from 'react'
import { Terminal } from 'lucide-react'

interface LogViewerProps {
  projectId: string
}

export default function LogViewer({ projectId }: LogViewerProps) {
  const [logs, setLogs] = useState<string[]>([])
  const [connected, setConnected] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const eventSource = new EventSource(`/api/projects/${projectId}/logs`)

    eventSource.onopen = () => setConnected(true)
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'log' && data.message) {
          setLogs((prev) => [...prev.slice(-500), data.message])
        }
      } catch {
        // ignore parse errors
      }
    }
    eventSource.onerror = () => {
      setConnected(false)
      eventSource.close()
    }

    return () => eventSource.close()
  }, [projectId])

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [logs])

  return (
    <div className="bg-slate-900 rounded-lg overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 bg-slate-800">
        <div className="flex items-center gap-2">
          <Terminal className="w-4 h-4 text-slate-400" />
          <span className="text-sm font-medium text-slate-300">Logs</span>
        </div>
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${connected ? 'bg-green-400' : 'bg-slate-500'}`} />
          <span className="text-xs text-slate-400">{connected ? 'Connected' : 'Disconnected'}</span>
        </div>
      </div>
      <div
        ref={containerRef}
        className="p-4 h-64 overflow-y-auto font-mono text-xs text-slate-300 leading-5"
      >
        {logs.length === 0 ? (
          <p className="text-slate-500">No logs available. Start the project to see logs.</p>
        ) : (
          logs.map((line, i) => (
            <div key={i} className="whitespace-pre-wrap break-all">
              {line}
            </div>
          ))
        )}
      </div>
    </div>
  )
}
