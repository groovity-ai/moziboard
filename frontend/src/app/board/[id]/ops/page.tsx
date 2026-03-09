'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import useSWR from 'swr';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { ActivitySquare, PlugZap, AlertTriangle, RefreshCw } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

type AgentEvent = {
  id: number;
  agent_id: string;
  board_id?: string;
  task_id?: number;
  event_type: string;
  delivery_status: string;
  delivery_attempts: number;
  next_attempt_at?: string;
  last_delivery_at?: string;
  response_status?: string;
  created_at: string;
};

type AgentConnector = {
  id: number;
  agent_id: string;
  connector_type: string;
  transport_mode: string;
  auth_type: string;
  endpoint_url?: string;
  agent_ref?: string;
  session_key?: string;
  status: string;
  last_success_at?: string;
  last_error?: string;
  updated_at: string;
};

function SummaryCard({ label, value, tone = 'default' }: { label: string; value: string; tone?: 'default' | 'danger' | 'success' }) {
  const toneClass = tone === 'danger'
    ? 'border-red-200 bg-red-50 dark:border-red-900/30 dark:bg-red-900/10'
    : tone === 'success'
    ? 'border-green-200 bg-green-50 dark:border-green-900/30 dark:bg-green-900/10'
    : 'border bg-card';

  return (
    <div className={`rounded-xl p-4 shadow-sm ${toneClass}`}>
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-2 text-2xl font-bold">{value}</div>
    </div>
  );
}

function StatusPill({ value }: { value?: string }) {
  const v = (value || 'unknown').toLowerCase();
  const cls = v === 'failed' || v === 'dead'
    ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    : v === 'sent' || v === 'connected'
    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    : v === 'processing' || v === 'routed'
    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    : 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300';
  return <span className={`rounded-full px-2 py-1 text-xs font-semibold ${cls}`}>{value || 'unknown'}</span>;
}

export default function OpsPage() {
  const params = useParams();
  const boardId = params.id as string;

  const [eventStatus, setEventStatus] = useState('');
  const [eventAgent, setEventAgent] = useState('');
  const [eventType, setEventType] = useState('');
  const [connectorStatus, setConnectorStatus] = useState('');
  const [connectorAgent, setConnectorAgent] = useState('');
  const [connectorType, setConnectorType] = useState('');

  const eventsUrl = useMemo(() => {
    const q = new URLSearchParams();
    q.set('board_id', boardId);
    if (eventStatus) q.set('status', eventStatus);
    if (eventAgent) q.set('agent_id', eventAgent);
    if (eventType) q.set('event_type', eventType);
    return `/api/ops/agent-events?${q.toString()}`;
  }, [boardId, eventStatus, eventAgent, eventType]);

  const connectorsUrl = useMemo(() => {
    const q = new URLSearchParams();
    if (connectorStatus) q.set('status', connectorStatus);
    if (connectorAgent) q.set('agent_id', connectorAgent);
    if (connectorType) q.set('connector_type', connectorType);
    q.set('transport_mode', 'internal');
    return `/api/ops/connectors?${q.toString()}`;
  }, [connectorStatus, connectorAgent, connectorType]);

  const { data: events = [], mutate: mutateEvents, isLoading: loadingEvents } = useSWR<AgentEvent[]>(eventsUrl, fetcher, { refreshInterval: 10000 });
  const { data: connectors = [], mutate: mutateConnectors, isLoading: loadingConnectors } = useSWR<AgentConnector[]>(connectorsUrl, fetcher, { refreshInterval: 10000 });

  const failedCount = events.filter((e) => e.delivery_status === 'failed' || e.delivery_status === 'dead').length;
  const activeConnectors = connectors.filter((c) => c.status === 'connected').length;
  const retryingCount = events.filter((e) => !!e.next_attempt_at && e.delivery_status === 'failed').length;

  return (
    <div className="flex h-full w-full flex-col">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
        <div className="flex items-center gap-2">
          <SidebarTrigger />
          <h1 className="flex items-center gap-2 px-2 text-lg font-bold tracking-tight">
            <ActivitySquare className="h-5 w-5 text-rose-500" />
            Ops Dashboard
          </h1>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            mutateEvents();
            mutateConnectors();
          }}
        >
          <RefreshCw className="mr-2 h-4 w-4" /> Refresh
        </Button>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="mx-auto max-w-7xl space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            <SummaryCard label="Failed / Dead Events" value={String(failedCount)} tone={failedCount > 0 ? 'danger' : 'success'} />
            <SummaryCard label="Retry Scheduled" value={String(retryingCount)} />
            <SummaryCard label="Connected Connectors" value={String(activeConnectors)} tone={activeConnectors > 0 ? 'success' : 'default'} />
          </div>

          <div className="rounded-2xl border bg-card p-4 shadow-sm">
            <div className="mb-4 flex items-center gap-2 text-sm font-semibold">
              <AlertTriangle className="h-4 w-4 text-amber-500" />
              Event Delivery
            </div>
            <div className="mb-4 grid gap-3 md:grid-cols-3">
              <Input placeholder="Filter status (failed, dead, sent)" value={eventStatus} onChange={(e) => setEventStatus(e.target.value)} />
              <Input placeholder="Filter agent_id" value={eventAgent} onChange={(e) => setEventAgent(e.target.value)} />
              <Input placeholder="Filter event_type" value={eventType} onChange={(e) => setEventType(e.target.value)} />
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="px-2 py-2">ID</th>
                    <th className="px-2 py-2">Agent</th>
                    <th className="px-2 py-2">Type</th>
                    <th className="px-2 py-2">Status</th>
                    <th className="px-2 py-2">Attempts</th>
                    <th className="px-2 py-2">Next Retry</th>
                    <th className="px-2 py-2">Response</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((ev) => (
                    <tr key={ev.id} className="border-b align-top">
                      <td className="px-2 py-2 font-mono text-xs">#{ev.id}</td>
                      <td className="px-2 py-2">{ev.agent_id}</td>
                      <td className="px-2 py-2">{ev.event_type}</td>
                      <td className="px-2 py-2"><StatusPill value={ev.delivery_status} /></td>
                      <td className="px-2 py-2">{ev.delivery_attempts}</td>
                      <td className="px-2 py-2 text-xs text-muted-foreground">{ev.next_attempt_at ? new Date(ev.next_attempt_at).toLocaleString() : '-'}</td>
                      <td className="px-2 py-2 text-xs text-muted-foreground max-w-[320px] truncate">{ev.response_status || '-'}</td>
                    </tr>
                  ))}
                  {!loadingEvents && events.length === 0 && (
                    <tr><td className="px-2 py-6 text-center text-muted-foreground" colSpan={7}>No events found.</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="rounded-2xl border bg-card p-4 shadow-sm">
            <div className="mb-4 flex items-center gap-2 text-sm font-semibold">
              <PlugZap className="h-4 w-4 text-rose-500" />
              Connectors
            </div>
            <div className="mb-4 grid gap-3 md:grid-cols-3">
              <Input placeholder="Filter status (connected, pending)" value={connectorStatus} onChange={(e) => setConnectorStatus(e.target.value)} />
              <Input placeholder="Filter agent_id" value={connectorAgent} onChange={(e) => setConnectorAgent(e.target.value)} />
              <Input placeholder="Filter connector_type" value={connectorType} onChange={(e) => setConnectorType(e.target.value)} />
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="px-2 py-2">ID</th>
                    <th className="px-2 py-2">Agent</th>
                    <th className="px-2 py-2">Type</th>
                    <th className="px-2 py-2">Transport</th>
                    <th className="px-2 py-2">Status</th>
                    <th className="px-2 py-2">Endpoint / Ref</th>
                    <th className="px-2 py-2">Updated</th>
                  </tr>
                </thead>
                <tbody>
                  {connectors.map((conn) => (
                    <tr key={conn.id} className="border-b align-top">
                      <td className="px-2 py-2 font-mono text-xs">#{conn.id}</td>
                      <td className="px-2 py-2">{conn.agent_id}</td>
                      <td className="px-2 py-2">{conn.connector_type}</td>
                      <td className="px-2 py-2">{conn.transport_mode}</td>
                      <td className="px-2 py-2"><StatusPill value={conn.status} /></td>
                      <td className="px-2 py-2 text-xs text-muted-foreground">{conn.endpoint_url || conn.agent_ref || conn.session_key || '-'}</td>
                      <td className="px-2 py-2 text-xs text-muted-foreground">{new Date(conn.updated_at).toLocaleString()}</td>
                    </tr>
                  ))}
                  {!loadingConnectors && connectors.length === 0 && (
                    <tr><td className="px-2 py-6 text-center text-muted-foreground" colSpan={7}>No connectors found.</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
