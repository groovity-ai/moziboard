'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import useSWR from 'swr';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { ActivitySquare, PlugZap, AlertTriangle, RefreshCw, ChevronRight, Shield } from 'lucide-react';
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

export default function OpsPage() {
  const params = useParams();
  const boardId = params.id as string;

  const [eventStatus, setEventStatus] = useState('');
  const [eventAgent, setEventAgent] = useState('');
  const [eventType, setEventType] = useState('');
  const [connectorStatus, setConnectorStatus] = useState('');
  const [connectorAgent, setConnectorAgent] = useState('');
  const [connectorType, setConnectorType] = useState('');
  const [selectedEvent, setSelectedEvent] = useState<AgentEvent | null>(null);
  const [selectedConnector, setSelectedConnector] = useState<AgentConnector | null>(null);
  const [eventActionLoading, setEventActionLoading] = useState<string | null>(null);
  const [connectorActionLoading, setConnectorActionLoading] = useState<string | null>(null);
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

  const { data: events = [], mutate: mutateEvents, isLoading: loadingEvents } = useSWR<AgentEvent[]>(eventsUrl, fetcher, { refreshInterval: 10000 });
  const { data: connectors = [], mutate: mutateConnectors, isLoading: loadingConnectors } = useSWR<AgentConnector[]>(connectorsUrl, fetcher, { refreshInterval: 10000 });

  const failedCount = events.filter((e) => e.delivery_status === 'failed' || e.delivery_status === 'dead').length;
  const activeConnectors = connectors.filter((c) => c.status === 'connected').length;
  const retryingCount = events.filter((e) => !!e.next_attempt_at && e.delivery_status === 'failed').length;

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
      const refreshed = (await fetcher(connectorsUrl)) as AgentConnector[];
      const updated = refreshed.find((item) => item.id === selectedConnector.id) || null;
      setSelectedConnector(updated);
    } catch (error) {
      setBanner({ tone: 'danger', text: error instanceof Error ? error.message : `Failed to ${action} connector.` });
    } finally {
      setConnectorActionLoading(null);
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
            {banner && (
              <div className={`rounded-xl border px-4 py-3 text-sm ${banner.tone === 'success' ? 'border-green-200 bg-green-50 text-green-800 dark:border-green-900/30 dark:bg-green-900/10 dark:text-green-300' : 'border-red-200 bg-red-50 text-red-800 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300'}`}>
                {banner.text}
              </div>
            )}

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
                    {events.map((ev) => (
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
                    {!loadingEvents && events.length === 0 && (
                      <tr><td className="px-2 py-6 text-center text-muted-foreground" colSpan={8}>No events found.</td></tr>
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
                    {connectors.map((conn) => (
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
                    {!loadingConnectors && connectors.length === 0 && (
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
