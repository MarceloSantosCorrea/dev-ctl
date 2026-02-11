import type { PortAllocation } from '../lib/api'

interface PortTableProps {
  ports: PortAllocation[]
}

export default function PortTable({ ports }: PortTableProps) {
  if (!ports || ports.length === 0) {
    return <p className="text-sm text-slate-500">Nenhuma porta alocada</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200">
            <th className="text-left py-2 px-3 font-medium text-slate-600">Porta Externa</th>
            <th className="text-left py-2 px-3 font-medium text-slate-600">Porta Interna</th>
            <th className="text-left py-2 px-3 font-medium text-slate-600">Protocolo</th>
          </tr>
        </thead>
        <tbody>
          {ports.map((port) => (
            <tr key={port.id} className="border-b border-slate-100">
              <td className="py-2 px-3 font-mono text-blue-600">{port.external_port}</td>
              <td className="py-2 px-3 font-mono text-slate-700">{port.internal_port}</td>
              <td className="py-2 px-3 text-slate-500 uppercase">{port.protocol}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
