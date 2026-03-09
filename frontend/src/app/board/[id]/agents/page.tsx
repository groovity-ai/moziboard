'use client';

import { useParams } from 'next/navigation';
import { Bot, Users, Activity, Bell, PlayCircle, Clock3, AlertTriangle } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import useSWR from 'swr';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

type Agent = {
  id: string;
  soul: string;
  memory: string;
  rules: string;
  cron_schedule: string;
  active: boolean;
  status: string;
  last_heartbeat_at?: string;
  last_seen_at?: string;
  current_task_id?: number;
  current_run_id?: number;
  current_activity?: string;
  health_note?: string;
};

type AgentRun = {
  id: number;
  task_id: number;
  agent_id: string;
  status: string;
  started_at: string;
  ended_at?: string;
  current_activity?: string;
  error_summary?: string;
  result_summary?: string;
};

type Notification = {
  id: number;
  type: string;
  content: string;
  delivered: boolean;
  created_at: string;
};

type Task = {
  id: number;
  title: string;
  status?: string;
  assignee_id?: string;
};

function StatusBadge({ status }: { status?: string }) {
  const normalized = (status || 'offline').toLowerCase();
  const map: Record<string, string> = {
    online: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    busy: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    blocked: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    offline: 'bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
  };

  return <span className={`rounded-full px-2 py-1 text-xs font-semibold ${map[normalized] || map.offline}`}>{normalized}</span>;
}

export default function AgentsPage() {
  const params = useParams();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);

  const { data: agents } = useSWR<Agent[]>('/api/agents', fetcher, { refreshInterval: 10000 });
  const { data: tasks } = useSWR<Task[]>(`/api/boards/${id}/tasks`, fetcher, { refreshInterval: 10000 });

  const boardAgents = (agents || []).filter((agent) =>
    (tasks || []).some((task) => task.assignee_id === agent.id)
  );
  const visibleAgents = boardAgents.length > 0 ? boardAgents : (agents || []).filter((agent) => agent.active);

  return (
    <div className="flex h-full w-full flex-col">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
        <div className="flex items-center gap-2">
          <SidebarTrigger />
          <h1 className="text-lg font-bold tracking-tight px-2 flex items-center gap-2">
            <Bot className="w-5 h-5 text-rose-500" />
            Agent Operations Center
          </h1>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowMembers(true)}
            className="flex items-center gap-2 rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium hover:bg-gray-200 dark:bg-zinc-800 dark:hover:bg-zinc-700"
          >
            <Users size={16} /> Members
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="mx-auto max-w-7xl space-y-6">
          <div className="grid gap-4 md:grid-cols-4">
            <SummaryCard icon={<Bot className="w-4 h-4" />} label="Active Agents" value={String(visibleAgents.length)} />
            <SummaryCard icon={<Activity className="w-4 h-4" />} label="Busy/Online" value={String(visibleAgents.filter((a) => ['busy', 'online'].includes((a.status || '').toLowerCase())).length)} />
            <SummaryCard icon={<Bell className="w-4 h-4" />} label="Blocked Agents" value={String(visibleAgents.filter((a) => (a.status || '').toLowerCase() === 'blocked').length)} />
            <SummaryCard icon={<Clock3 className="w-4 h-4" />} label="Assigned Tasks" value={String((tasks || []).filter((t) => !!t.assignee_id && t.status !== 'done').length)} />
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            {visibleAgents.map((agent) => (
              <AgentCard key={agent.id} agent={agent} tasks={tasks || []} />
            ))}
          </div>

          {visibleAgents.length === 0 && (
            <div className="rounded-xl border border-dashed p-10 text-center text-sm text-muted-foreground">
              Belum ada agent yang aktif di board ini.
            </div>
          )}
        </div>
      </div>

      <MemberManager boardId={id} isOpen={showMembers} onClose={() => setShowMembers(false)} />
    </div>
  );
}

function SummaryCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-xl border bg-card p-4 shadow-sm">
      <div className="mb-2 flex items-center gap-2 text-sm text-muted-foreground">{icon}{label}</div>
      <div className="text-2xl font-bold">{value}</div>
    </div>
  );
}

function AgentCard({ agent, tasks }: { agent: Agent; tasks: Task[] }) {
  const { data: runs } = useSWR<AgentRun[]>(`/api/agents/${agent.id}/runs`, fetcher, { refreshInterval: 10000 });
  const { data: notifications } = useSWR<Notification[]>(`/api/agents/${agent.id}/notifications`, fetcher, { refreshInterval: 10000 });
  const currentTask = tasks.find((task) => task.id === agent.current_task_id || task.assignee_id === agent.id);
  const pendingNotifications = (notifications || []).filter((n) => !n.delivered);
  const latestRun = runs?.[0];

  return (
    <div className="rounded-2xl border bg-card p-5 shadow-sm">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-rose-100 text-lg dark:bg-rose-900/30">🤖</div>
            <div>
              <div className="font-semibold text-lg">{agent.id}</div>
              <div className="text-xs text-muted-foreground">Heartbeat: {agent.cron_schedule}</div>
            </div>
          </div>
        </div>
        <StatusBadge status={agent.status} />
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <InfoRow label="Current Task" value={currentTask ? `${currentTask.title} (#${currentTask.id})` : 'Idle'} />
        <InfoRow label="Current Activity" value={agent.current_activity || latestRun?.current_activity || '-'} />
        <InfoRow label="Last Heartbeat" value={agent.last_heartbeat_at ? new Date(agent.last_heartbeat_at).toLocaleString() : '-'} />
        <InfoRow label="Health Note" value={agent.health_note || latestRun?.error_summary || '-'} />
      </div>

      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <div className="rounded-xl bg-muted/40 p-4">
          <div className="mb-2 flex items-center gap-2 text-sm font-semibold"><PlayCircle className="w-4 h-4" /> Latest Run</div>
          {latestRun ? (
            <div className="space-y-1 text-sm">
              <div><span className="text-muted-foreground">Status:</span> {latestRun.status}</div>
              <div><span className="text-muted-foreground">Task:</span> #{latestRun.task_id}</div>
              <div><span className="text-muted-foreground">Started:</span> {new Date(latestRun.started_at).toLocaleString()}</div>
              {latestRun.result_summary && <div className="text-muted-foreground">{latestRun.result_summary}</div>}
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">Belum ada run history.</div>
          )}
        </div>

        <div className="rounded-xl bg-muted/40 p-4">
          <div className="mb-2 flex items-center gap-2 text-sm font-semibold"><Bell className="w-4 h-4" /> Pending Signals</div>
          {pendingNotifications.length > 0 ? (
            <div className="space-y-2">
              {pendingNotifications.slice(0, 3).map((notif) => (
                <div key={notif.id} className="rounded-lg bg-background p-2 text-sm border">
                  <div className="font-medium">{notif.type.replace(/_/g, ' ')}</div>
                  <div className="text-muted-foreground text-xs">{notif.content}</div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">Tidak ada notifikasi pending.</div>
          )}
        </div>
      </div>

      {(agent.status || '').toLowerCase() === 'blocked' && (
        <div className="mt-4 flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">
          <AlertTriangle className="w-4 h-4 mt-0.5" />
          <div>{agent.health_note || 'Agent sedang blocked dan butuh perhatian.'}</div>
        </div>
      )}
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border p-3">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="mt-1 text-sm font-medium">{value}</div>
    </div>
  );
}
