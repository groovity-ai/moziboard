'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import useSWR from 'swr';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { ActivitySquare, PlugZap, AlertTriangle, RefreshCw, ChevronRight, Shield, Wrench, RadioTower, Bot, Clock3, ScrollText } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer';

const fetcher = async (url: string) => {
  const res = await fetch(url);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
};

type AgentEvent = {
  id: number;
  agent_id: string;
  board_id?: string;
  task_id?: number;
  event_type: string;
  payload_json?: string;
  delivery_status: string;
  delivery_attempts: number;
  next_attempt_at?: string;
  last_delivery_at?: string;
  response_status?: string;
  response_body?: string;
  created_at: string;
  processed_at?: string;
};

type AgentConnector = {
  id: number;
  agent_id: string;
  connector_type: string;
  transport_mode: string;
  auth_type: string;
  endpoint_url?: string;
  base_url?: string;
  agent_ref?: string;
  session_key?: string;
  status: string;
  metadata_json?: string;
  last_success_at?: string;
  last_error?: string;
  updated_at: string;
  created_at?: string;
};

type AuditLog = {
  id: number;
  actor_type: string;
  actor_id: string;
  entity_type: string;
  entity_id: string;
  action: string;
  old_value_json?: string;
  new_value_json?: string;
  metadata_json?: string;
  created_at: string;
};

type Readiness = {
  status: string;
  time: string;
  checks: {
    db?: { ok: boolean; error?: string };
    redis?: { ok: boolean; error?: string };
    build?: { ok: boolean; version?: string; go_env?: string };
  };
  latency?: Record<string, { last_ms?: number; avg_ms?: number; max_ms?: number; count?: number; last_status?: number; updated_at?: string }>;
};

type OpsSummary = {
  generated_at: string;
  scope?: {
    board_id?: string;
    mode?: string;
  };
  agents: {
    total: number;
    online: number;
    offline: number;
    stale: number;
    blocked: number;
    with_issues: number;
  };
  runs: {
    total: number;
    active: number;
    stale: number;
    failed: number;
    review: number;
    blocked: number;
  };
  events: {
    pending: number;
    processing: number;
    failed: number;
    dead: number;
    sent: number;
    routed: number;
    ignored: number;
    retry_due: number;
  };
  connectors: {
    total: number;
    connected: number;
    disabled: number;
    webhook: number;
    internal: number;
    with_errors: number;
  };
  audit_logs: {
    total_24h: number;
    maintenance_24h: number;
    ops_actions_24h: number;
    runtime_events_24h: number;
  };
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
  const cls = v === 'failed' || v === 'dead' || v === 'disabled'
    ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    : v === 'sent' || v === 'connected'
    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    : v === 'processing' || v === 'routed'
    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    : 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300';
  return <span className={`rounded-full px-2 py-1 text-xs font-semibold ${cls}`}>{value || 'unknown'}</span>;
}

function PrettyJson({ value }: { value?: string }) {
  if (!value) return <div className="text-sm text-muted-foreground">-</div>;
  try {
    const parsed = JSON.parse(value);
    return <pre className="overflow-x-auto rounded-lg bg-muted/50 p-3 text-xs">{JSON.stringify(parsed, null, 2)}</pre>;
  } catch {
    return <pre className="overflow-x-auto rounded-lg bg-muted/50 p-3 text-xs whitespace-pre-wrap">{value}</pre>;
  }
}

function DetailRow({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <div className="rounded-lg border p-3">
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="mt-1 text-sm break-all">{value ?? '-'}</div>
    </div>
  );
}

function getOpsSeverity(summary?: OpsSummary, readiness?: Readiness): { label: string; tone: string; reason: string } {
  if (!summary) return { label: 'Loading', tone: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300', reason: 'Menunggu snapshot operasional.' };

  const latencyEntries = Object.entries(readiness?.latency || {});
  const maxAvgLatency = latencyEntries.reduce((max, [, stats]) => Math.max(max, stats?.avg_ms || 0), 0);
  const maxLastLatency = latencyEntries.reduce((max, [, stats]) => Math.max(max, stats?.last_ms || 0), 0);

  if (
    readiness?.status === 'degraded' ||
    summary.events.dead > 0 ||
    summary.runs.stale > 0 ||
    summary.events.failed >= 3 ||
    maxAvgLatency >= 1200 ||
    maxLastLatency >= 2000
  ) {
    return {
      label: 'Critical',
      tone: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
      reason: readiness?.status === 'degraded'
        ? 'Dependency backend lagi degraded.'
        : summary.events.dead > 0
        ? 'Ada dead event yang butuh intervensi.'
        : summary.runs.stale > 0
        ? 'Ada stale run yang berpotensi nyangkut.'
        : maxAvgLatency >= 1200 || maxLastLatency >= 2000
        ? 'Latency endpoint penting lagi tinggi.'
        : 'Error operasional sudah masuk level kritis.',
    };
  }

  if (
    summary.events.retry_due > 0 ||
    summary.agents.with_issues > 0 ||
    summary.runs.failed > 0 ||
    summary.connectors.with_errors > 0 ||
    summary.connectors.disabled > 0 ||
    maxAvgLatency >= 500 ||
    maxLastLatency >= 900
  ) {
    return {
      label: 'Warning',
      tone: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
      reason: summary.events.retry_due > 0
        ? 'Ada retry backlog yang perlu dipantau.'
        : summary.connectors.with_errors > 0 || summary.connectors.disabled > 0
        ? 'Ada connector error/disabled.'
        : maxAvgLatency >= 500 || maxLastLatency >= 900
        ? 'Latency mulai naik tapi belum kritis.'
        : 'Ada sinyal operasional yang perlu perhatian.',
    };
  }

  return { label: 'Healthy', tone: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300', reason: 'Belum ada sinyal operasional yang mengkhawatirkan.' };
}

export default function OpsPage() {
  const params = useParams();
  const boardId = params.id as string;

  const [eventStatus, setEventStatus] = useState('');
  const [eventAgent, setEventAgent] = useState('');
  const [eventType, setEventType] = useState('');
  const [showOnlyUnhealthyEvents, setShowOnlyUnhealthyEvents] = useState(false);
  const [connectorStatus, setConnectorStatus] = useState('');
  const [connectorAgent, setConnectorAgent] = useState('');
  const [connectorType, setConnectorType] = useState('');
  const [showOnlyUnhealthyConnectors, setShowOnlyUnhealthyConnectors] = useState(false);
  const [selectedEvent, setSelectedEvent] = useState<AgentEvent | null>(null);
  const [selectedConnector, setSelectedConnector] = useState<AgentConnector | null>(null);
  const [eventActionLoading, setEventActionLoading] = useState<string | null>(null);
  const [connectorActionLoading, setConnectorActionLoading] = useState<string | null>(null);
  const [maintenanceLoading, setMaintenanceLoading] = useState(false);
  const [banner, setBanner] = useState<{ tone: 'success' | 'danger'; text: string } | null>(null);

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

  const summaryUrl = useMemo(() => `/api/ops/summary?board_id=${encodeURIComponent(boardId)}`, [boardId]);
  const readinessUrl = useMemo(() => '/api/readiness', []);

  const auditUrl = useMemo(() => `/api/ops/audit-logs?board_id=${encodeURIComponent(boardId)}`, [boardId]);

  const { data: summary, mutate: mutateSummary, isLoading: loadingSummary } = useSWR<OpsSummary>(summaryUrl, fetcher, { refreshInterval: 10000 });
  const { data: readiness, mutate: mutateReadiness, isLoading: loadingReadiness } = useSWR<Readiness>(readinessUrl, fetcher, { refreshInterval: 15000 });
  const { data: events = [], mutate: mutateEvents, isLoading: loadingEvents } = useSWR<AgentEvent[]>(eventsUrl, fetcher, { refreshInterval: 10000 });
  const { data: connectors = [], mutate: mutateConnectors, isLoading: loadingConnectors } = useSWR<AgentConnector[]>(connectorsUrl, fetcher, { refreshInterval: 10000 });
  const { data: auditLogs = [], mutate: mutateAuditLogs, isLoading: loadingAuditLogs } = useSWR<AuditLog[]>(auditUrl, fetcher, { refreshInterval: 15000 });

  const failedCount = events.filter((e) => e.delivery_status === 'failed' || e.delivery_status === 'dead').length;
  const activeConnectors = connectors.filter((c) => c.status === 'connected').length;
  const retryingCount = events.filter((e) => !!e.next_attempt_at && e.delivery_status === 'failed').length;
  const severity = getOpsSeverity(summary, readiness);
  const visibleEvents = showOnlyUnhealthyEvents ? events.filter((e) => ['failed', 'dead', 'processing'].includes(e.delivery_status) || (!!e.next_attempt_at && e.delivery_status === 'failed')) : events;
  const visibleConnectors = showOnlyUnhealthyConnectors ? connectors.filter((c) => c.status === 'disabled' || !!c.last_error) : connectors;
  const visibleAuditLogs = auditLogs.filter((log) => ['maintenance', 'agent_event', 'connector'].includes(log.entity_type) || ['maintenance_sweep_ran', 'retry_requested', 'requeue_requested', 'connector_enabled', 'connector_disabled'].includes(log.action)).slice(0, 20);
  const latencyEntries = Object.entries(readiness?.latency || {}).filter(([path]) => path === '/api/readiness' || path === '/api/home/overview' || path.startsWith('/api/ops/')).slice(0, 6);

  async function runEventAction(action: 'retry' | 'requeue') {
    if (!selectedEvent) return;
    const key = `${action}:${selectedEvent.id}`;
    setEventActionLoading(key);
    setBanner(null);
    try {
      const res = await fetch(`/api/ops/agent-events/${selectedEvent.id}/${action}`, { method: 'POST' });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error || `${action} failed`);
      setBanner({ tone: 'success', text: `Event #${selectedEvent.id} ${action} requested.` });
      await mutateEvents();
      await mutateAuditLogs();
      const refreshed = (await fetcher(eventsUrl)) as AgentEvent[];
      const updated = refreshed.find((item) => item.id === selectedEvent.id) || null;
      setSelectedEvent(updated);
    } catch (error) {
      setBanner({ tone: 'danger', text: error instanceof Error ? error.message : `Failed to ${action} event.` });
    } finally {
      setEventActionLoading(null);
    }
  }

  async function runConnectorAction(action: 'enable' | 'disable') {
    if (!selectedConnector) return;
    const key = `${action}:${selectedConnector.id}`;
    setConnectorActionLoading(key);
    setBanner(null);
    try {
      const res = await fetch(`/api/ops/connectors/${selectedConnector.id}/${action}`, { method: 'POST' });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error || `${action} failed`);
      setBanner({ tone: 'success', text: `Connector #${selectedConnector.id} ${action}d.` });
      await mutateConnectors();
      await mutateSummary();
      await mutateAuditLogs();
      const refreshed = (await fetcher(connectorsUrl)) as AgentConnector[];
      const updated = refreshed.find((item) => item.id === selectedConnector.id) || null;
      setSelectedConnector(updated);
    } catch (error) {
      setBanner({ tone: 'danger', text: error instanceof Error ? error.message : `Failed to ${action} connector.` });
    } finally {
      setConnectorActionLoading(null);
    }
  }

  async function runMaintenanceSweep() {
    setMaintenanceLoading(true);
    setBanner(null);
    try {
      const res = await fetch('/api/ops/maintenance/run', { method: 'POST' });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body.error || 'maintenance run failed');
      const report = body.report || {};
      setBanner({
        tone: 'success',
        text: `Maintenance sweep jalan: stale_agents=${report.stale_agents_marked ?? 0}, stale_runs=${report.stale_runs_closed ?? 0}, repaired=${report.drifted_tasks_repaired ?? 0}`,
      });
      await Promise.all([mutateSummary(), mutateEvents(), mutateConnectors(), mutateAuditLogs()]);
    } catch (error) {
      setBanner({ tone: 'danger', text: error instanceof Error ? error.message : 'Failed to run maintenance sweep.' });
    } finally {
      setMaintenanceLoading(false);
    }
  }

  return (
    <>
      <div className="flex h-full w-full flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
          <div className="flex items-center gap-2">
            <SidebarTrigger />
            <h1 className="flex items-center gap-2 px-2 text-lg font-bold tracking-tight">
              <ActivitySquare className="h-5 w-5 text-rose-500" />
              Ops Dashboard
            </h1>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={runMaintenanceSweep} disabled={maintenanceLoading}>
              <Wrench className="mr-2 h-4 w-4" /> {maintenanceLoading ? 'Running…' : 'Run maintenance'}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                mutateSummary();
                mutateReadiness();
                mutateEvents();
                mutateConnectors();
                mutateAuditLogs();
              }}
            >
              <RefreshCw className="mr-2 h-4 w-4" /> Refresh
            </Button>
          </div>
        </header>

        <div className="flex-1 overflow-auto p-6">
          <div className="mx-auto max-w-7xl space-y-6">
            {banner && (
              <div className={`rounded-xl border px-4 py-3 text-sm ${banner.tone === 'success' ? 'border-green-200 bg-green-50 text-green-800 dark:border-green-900/30 dark:bg-green-900/10 dark:text-green-300' : 'border-red-200 bg-red-50 text-red-800 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300'}`}>
                {banner.text}
              </div>
            )}

            <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
              <div className="rounded-2xl border bg-card p-4 shadow-sm">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-sm font-semibold">Backend Readiness</div>
                    <div className="mt-1 text-xs text-muted-foreground">Public infra check for backend dependencies.</div>
                  </div>
                  <span className={`rounded-full px-2 py-1 text-[11px] font-semibold ${readiness?.status === 'ready' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300' : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'}`}>
                    {loadingReadiness ? 'Loading' : readiness?.status || 'unknown'}
                  </span>
                </div>
                <div className="mt-4 grid grid-cols-3 gap-3 text-sm">
                  <div className="rounded-xl border p-3">
                    <div className="text-muted-foreground">DB</div>
                    <div className={`mt-1 text-lg font-bold ${readiness?.checks?.db?.ok ? 'text-green-600 dark:text-green-300' : 'text-red-600 dark:text-red-300'}`}>{readiness?.checks?.db?.ok ? 'OK' : loadingReadiness ? '…' : 'Fail'}</div>
                  </div>
                  <div className="rounded-xl border p-3">
                    <div className="text-muted-foreground">Redis</div>
                    <div className={`mt-1 text-lg font-bold ${readiness?.checks?.redis?.ok ? 'text-green-600 dark:text-green-300' : 'text-red-600 dark:text-red-300'}`}>{readiness?.checks?.redis?.ok ? 'OK' : loadingReadiness ? '…' : 'Fail'}</div>
                  </div>
                  <div className="rounded-xl border p-3">
                    <div className="text-muted-foreground">Build</div>
                    <div className="mt-1 text-sm font-semibold">{readiness?.checks?.build?.version || 'moziboard-backend'}</div>
                    <div className="text-xs text-muted-foreground">{readiness?.checks?.build?.go_env || 'default env'}</div>
                  </div>
                </div>
                <div className="mt-4 rounded-xl border p-3">
                  <div className="text-sm font-medium">Latency hints</div>
                  <div className="mt-2 space-y-2 text-xs text-muted-foreground">
                    {latencyEntries.length > 0 ? latencyEntries.map(([path, stats]) => (
                      <div key={path} className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2">
                        <div className="truncate font-mono text-[11px]">{path}</div>
                        <div className="flex items-center gap-3 whitespace-nowrap">
                          <span>last {stats.last_ms ?? '-'}ms</span>
                          <span>avg {stats.avg_ms ?? '-'}ms</span>
                          <span>max {stats.max_ms ?? '-'}ms</span>
                        </div>
                      </div>
                    )) : <div>{loadingReadiness ? 'Collecting latency…' : 'No latency samples yet.'}</div>}
                  </div>
                </div>
                <div className="mt-3 text-xs text-muted-foreground">
                  {readiness?.time ? `Checked ${new Date(readiness.time).toLocaleString()}` : loadingReadiness ? 'Checking readiness…' : 'Readiness unavailable'}
                </div>
              </div>

              <div className="rounded-2xl border bg-card p-4 shadow-sm">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <div className="text-sm font-semibold">Ops Summary</div>
                    <span className={`rounded-full px-2 py-1 text-[11px] font-semibold ${severity.tone}`}>{severity.label}</span>
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    Snapshot operasional khusus board ini: runtime, delivery, connector, dan audit surface.
                  </div>
                  <div className="mt-2 text-xs text-muted-foreground">
                    {severity.reason}
                  </div>
                </div>
                <div className="text-right text-xs text-muted-foreground">
                  <div>{summary?.generated_at ? `Updated ${new Date(summary.generated_at).toLocaleString()}` : loadingSummary ? 'Loading summary…' : 'Summary unavailable'}</div>
                  <div className="mt-1">Scope: board {boardId}</div>
                </div>
              </div>

              <div className="mt-4 grid gap-4 md:grid-cols-2">
                <div className="rounded-xl border p-4">
                  <div className="flex items-center gap-2 text-sm font-medium"><Bot className="h-4 w-4 text-violet-500" /> Agents</div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                    <div><div className="text-muted-foreground">Total</div><div className="text-xl font-bold">{summary?.agents.total ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Online</div><div className="text-xl font-bold text-green-600 dark:text-green-300">{summary?.agents.online ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Offline</div><div className="text-xl font-bold">{summary?.agents.offline ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Issues</div><div className="text-xl font-bold text-amber-600 dark:text-amber-300">{summary?.agents.with_issues ?? '-'}</div></div>
                  </div>
                </div>

                <div className="rounded-xl border p-4">
                  <div className="flex items-center gap-2 text-sm font-medium"><Clock3 className="h-4 w-4 text-amber-500" /> Runs</div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                    <div><div className="text-muted-foreground">Active</div><div className="text-xl font-bold">{summary?.runs.active ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Failed</div><div className="text-xl font-bold text-red-600 dark:text-red-300">{summary?.runs.failed ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Review</div><div className="text-xl font-bold">{summary?.runs.review ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Stale</div><div className="text-xl font-bold text-amber-600 dark:text-amber-300">{summary?.runs.stale ?? '-'}</div></div>
                  </div>
                </div>

                <div className="rounded-xl border p-4">
                  <div className="flex items-center gap-2 text-sm font-medium"><RadioTower className="h-4 w-4 text-rose-500" /> Events</div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                    <div><div className="text-muted-foreground">Pending</div><div className="text-xl font-bold">{summary?.events.pending ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Retry Due</div><div className="text-xl font-bold text-amber-600 dark:text-amber-300">{summary?.events.retry_due ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Failed</div><div className="text-xl font-bold text-red-600 dark:text-red-300">{summary?.events.failed ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Dead</div><div className="text-xl font-bold text-red-600 dark:text-red-300">{summary?.events.dead ?? '-'}</div></div>
                  </div>
                </div>

                <div className="rounded-xl border p-4">
                  <div className="flex items-center gap-2 text-sm font-medium"><Shield className="h-4 w-4 text-emerald-500" /> Connectors & Audit</div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-sm">
                    <div><div className="text-muted-foreground">Connected</div><div className="text-xl font-bold text-green-600 dark:text-green-300">{summary?.connectors.connected ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Errors</div><div className="text-xl font-bold text-amber-600 dark:text-amber-300">{summary?.connectors.with_errors ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Maintenance 24h</div><div className="text-xl font-bold">{summary?.audit_logs.maintenance_24h ?? '-'}</div></div>
                    <div><div className="text-muted-foreground">Audit 24h</div><div className="text-xl font-bold">{summary?.audit_logs.total_24h ?? '-'}</div></div>
                  </div>
                </div>
              </div>
              </div>
            </div>

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
              <div className="mb-3 flex flex-wrap gap-2">
                {['', 'failed', 'dead', 'sent', 'processing'].map((preset) => (
                  <Button key={preset || 'all-events'} variant={eventStatus === preset ? 'default' : 'outline'} size="sm" onClick={() => setEventStatus(preset)}>
                    {preset || 'All'}
                  </Button>
                ))}
                <Button variant={showOnlyUnhealthyEvents ? 'default' : 'outline'} size="sm" onClick={() => setShowOnlyUnhealthyEvents((v) => !v)}>
                  Only unhealthy
                </Button>
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
                      <th className="px-2 py-2"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleEvents.map((ev) => (
                      <tr key={ev.id} className="cursor-pointer border-b align-top hover:bg-muted/30" onClick={() => setSelectedEvent(ev)}>
                        <td className="px-2 py-2 font-mono text-xs">#{ev.id}</td>
                        <td className="px-2 py-2">{ev.agent_id}</td>
                        <td className="px-2 py-2">{ev.event_type}</td>
                        <td className="px-2 py-2"><StatusPill value={ev.delivery_status} /></td>
                        <td className="px-2 py-2">{ev.delivery_attempts}</td>
                        <td className="px-2 py-2 text-xs text-muted-foreground">{ev.next_attempt_at ? new Date(ev.next_attempt_at).toLocaleString() : '-'}</td>
                        <td className="max-w-[320px] truncate px-2 py-2 text-xs text-muted-foreground">{ev.response_status || '-'}</td>
                        <td className="px-2 py-2 text-right text-muted-foreground"><ChevronRight className="inline h-4 w-4" /></td>
                      </tr>
                    ))}
                    {!loadingEvents && visibleEvents.length === 0 && (
                      <tr><td className="px-2 py-6 text-center text-muted-foreground" colSpan={8}>No events found.</td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="rounded-2xl border bg-card p-4 shadow-sm">
              <div className="mb-4 flex items-center gap-2 text-sm font-semibold">
                <ScrollText className="h-4 w-4 text-sky-500" />
                Recent Audit Trail
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="px-2 py-2">Time</th>
                      <th className="px-2 py-2">Actor</th>
                      <th className="px-2 py-2">Entity</th>
                      <th className="px-2 py-2">Action</th>
                      <th className="px-2 py-2">Metadata</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleAuditLogs.map((log) => (
                      <tr key={log.id} className="border-b align-top">
                        <td className="px-2 py-2 text-xs text-muted-foreground">{new Date(log.created_at).toLocaleString()}</td>
                        <td className="px-2 py-2"><div className="font-medium">{log.actor_id}</div><div className="text-xs text-muted-foreground">{log.actor_type}</div></td>
                        <td className="px-2 py-2"><div>{log.entity_type}</div><div className="text-xs text-muted-foreground break-all">{log.entity_id}</div></td>
                        <td className="px-2 py-2"><Badge variant="outline">{log.action}</Badge></td>
                        <td className="max-w-[360px] px-2 py-2 text-xs text-muted-foreground"><div className="line-clamp-3 break-all">{log.metadata_json || log.new_value_json || '-'}</div></td>
                      </tr>
                    ))}
                    {!loadingAuditLogs && visibleAuditLogs.length === 0 && (
                      <tr><td className="px-2 py-6 text-center text-muted-foreground" colSpan={5}>No audit logs found.</td></tr>
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
              <div className="mb-3 flex flex-wrap gap-2">
                {['', 'connected', 'pending', 'failed', 'disabled'].map((preset) => (
                  <Button key={preset || 'all-connectors'} variant={connectorStatus === preset ? 'default' : 'outline'} size="sm" onClick={() => setConnectorStatus(preset)}>
                    {preset || 'All'}
                  </Button>
                ))}
                <Button variant={showOnlyUnhealthyConnectors ? 'default' : 'outline'} size="sm" onClick={() => setShowOnlyUnhealthyConnectors((v) => !v)}>
                  Only unhealthy
                </Button>
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
                      <th className="px-2 py-2">Auth</th>
                      <th className="px-2 py-2">Status</th>
                      <th className="px-2 py-2">Endpoint / Ref</th>
                      <th className="px-2 py-2"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleConnectors.map((conn) => (
                      <tr key={conn.id} className="cursor-pointer border-b align-top hover:bg-muted/30" onClick={() => setSelectedConnector(conn)}>
                        <td className="px-2 py-2 font-mono text-xs">#{conn.id}</td>
                        <td className="px-2 py-2">{conn.agent_id}</td>
                        <td className="px-2 py-2">{conn.connector_type}</td>
                        <td className="px-2 py-2">{conn.transport_mode}</td>
                        <td className="px-2 py-2"><Badge variant="outline">{conn.auth_type}</Badge></td>
                        <td className="px-2 py-2"><StatusPill value={conn.status} /></td>
                        <td className="px-2 py-2 text-xs text-muted-foreground">{conn.endpoint_url || conn.agent_ref || conn.session_key || '-'}</td>
                        <td className="px-2 py-2 text-right text-muted-foreground"><ChevronRight className="inline h-4 w-4" /></td>
                      </tr>
                    ))}
                    {!loadingConnectors && visibleConnectors.length === 0 && (
                      <tr><td className="px-2 py-6 text-center text-muted-foreground" colSpan={8}>No connectors found.</td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      </div>

      <Drawer open={!!selectedEvent} onOpenChange={(open) => !open && setSelectedEvent(null)} direction="right">
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>Event Detail #{selectedEvent?.id}</DrawerTitle>
            <DrawerDescription>Inspect delivery state, retry metadata, and response details.</DrawerDescription>
          </DrawerHeader>
          {selectedEvent && (
            <div className="space-y-4 overflow-auto p-4">
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  onClick={() => runEventAction('retry')}
                  disabled={!['failed', 'processing'].includes(selectedEvent.delivery_status) || eventActionLoading !== null}
                >
                  {eventActionLoading === `retry:${selectedEvent.id}` ? 'Retrying...' : 'Retry now'}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => runEventAction('requeue')}
                  disabled={!['dead', 'failed', 'ignored', 'routed'].includes(selectedEvent.delivery_status) || eventActionLoading !== null}
                >
                  {eventActionLoading === `requeue:${selectedEvent.id}` ? 'Requeueing...' : 'Requeue'}
                </Button>
              </div>
              <div className="grid gap-3">
                <DetailRow label="Agent" value={selectedEvent.agent_id} />
                <DetailRow label="Event Type" value={selectedEvent.event_type} />
                <DetailRow label="Task ID" value={selectedEvent.task_id} />
                <DetailRow label="Board ID" value={selectedEvent.board_id} />
                <DetailRow label="Delivery Status" value={selectedEvent.delivery_status} />
                <DetailRow label="Attempts" value={selectedEvent.delivery_attempts} />
                <DetailRow label="Created At" value={selectedEvent.created_at ? new Date(selectedEvent.created_at).toLocaleString() : '-'} />
                <DetailRow label="Last Delivery" value={selectedEvent.last_delivery_at ? new Date(selectedEvent.last_delivery_at).toLocaleString() : '-'} />
                <DetailRow label="Next Retry" value={selectedEvent.next_attempt_at ? new Date(selectedEvent.next_attempt_at).toLocaleString() : '-'} />
                <DetailRow label="Processed At" value={selectedEvent.processed_at ? new Date(selectedEvent.processed_at).toLocaleString() : '-'} />
                <DetailRow label="Response Status" value={selectedEvent.response_status} />
              </div>
              <div>
                <div className="mb-2 text-sm font-semibold">Payload JSON</div>
                <PrettyJson value={selectedEvent.payload_json} />
              </div>
              <div>
                <div className="mb-2 text-sm font-semibold">Response Body</div>
                <PrettyJson value={selectedEvent.response_body} />
              </div>
            </div>
          )}
        </DrawerContent>
      </Drawer>

      <Drawer open={!!selectedConnector} onOpenChange={(open) => !open && setSelectedConnector(null)} direction="right">
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>Connector Detail #{selectedConnector?.id}</DrawerTitle>
            <DrawerDescription>Inspect connector identity, transport, auth, and health fields.</DrawerDescription>
          </DrawerHeader>
          {selectedConnector && (
            <div className="space-y-4 overflow-auto p-4">
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  onClick={() => runConnectorAction('enable')}
                  disabled={selectedConnector.status === 'connected' || connectorActionLoading !== null}
                >
                  {connectorActionLoading === `enable:${selectedConnector.id}` ? 'Enabling...' : 'Enable connector'}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => runConnectorAction('disable')}
                  disabled={selectedConnector.status === 'disabled' || connectorActionLoading !== null}
                >
                  {connectorActionLoading === `disable:${selectedConnector.id}` ? 'Disabling...' : 'Disable connector'}
                </Button>
              </div>
              <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                <Shield className="h-4 w-4" /> Auth & routing
              </div>
              <div className="grid gap-3">
                <DetailRow label="Agent" value={selectedConnector.agent_id} />
                <DetailRow label="Connector Type" value={selectedConnector.connector_type} />
                <DetailRow label="Transport Mode" value={selectedConnector.transport_mode} />
                <DetailRow label="Auth Type" value={selectedConnector.auth_type} />
                <DetailRow label="Status" value={selectedConnector.status} />
                <DetailRow label="Endpoint URL" value={selectedConnector.endpoint_url} />
                <DetailRow label="Base URL" value={selectedConnector.base_url} />
                <DetailRow label="Agent Ref" value={selectedConnector.agent_ref} />
                <DetailRow label="Session Key" value={selectedConnector.session_key} />
                <DetailRow label="Last Success" value={selectedConnector.last_success_at ? new Date(selectedConnector.last_success_at).toLocaleString() : '-'} />
                <DetailRow label="Updated At" value={selectedConnector.updated_at ? new Date(selectedConnector.updated_at).toLocaleString() : '-'} />
                <DetailRow label="Last Error" value={selectedConnector.last_error} />
              </div>
              <div>
                <div className="mb-2 text-sm font-semibold">Metadata JSON</div>
                <PrettyJson value={selectedConnector.metadata_json} />
              </div>
            </div>
          )}
        </DrawerContent>
      </Drawer>
    </>
  );
}
