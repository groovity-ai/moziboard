'use client';

import { useParams } from 'next/navigation';
import { Bot, Users, Activity, Bell, PlayCircle, Clock3, AlertTriangle, Cpu, PlugZap } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { MemberManager } from '@/components/MemberManager';
import { useMemo, useState } from 'react';
import useSWR from 'swr';
import { RegisterAgentModal } from '@/components/RegisterAgentModal';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

type Agent = {
  id: string;
  display_name?: string;
  role_name?: string;
  avatar?: string;
  provider?: string;
  engine?: string;
  description?: string;
  is_native_clawn?: boolean;
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

type BoardAgent = {
  id: number;
  board_id: string;
  agent_id: string;
  board_role: string;
  active: boolean;
  auto_accept_tasks: boolean;
  can_comment: boolean;
  can_update_status: boolean;
  can_access_docs: boolean;
  can_create_deliverables: boolean;
  capabilities_json?: string;
  agent?: Agent;
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
  const [showRegister, setShowRegister] = useState(false);

  const { data: boardAgents, mutate: mutateBoardAgents } = useSWR<BoardAgent[]>(`/api/boards/${id}/agents`, fetcher, { refreshInterval: 10000 });
  const { data: tasks } = useSWR<Task[]>(`/api/boards/${id}/tasks`, fetcher, { refreshInterval: 10000 });

  const visibleAgents = useMemo(() => (boardAgents || []).filter((entry) => entry.active !== false), [boardAgents]);
  const clawnAgents = visibleAgents.filter((entry) => entry.agent?.is_native_clawn);
  const connectedAgents = visibleAgents.filter((entry) => !!entry.agent);

  return (
    <div className="flex h-full w-full flex-col">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
        <div className="flex items-center gap-2">
          <SidebarTrigger />
          <h1 className="flex items-center gap-2 px-2 text-lg font-bold tracking-tight">
            <Bot className="h-5 w-5 text-rose-500" />
            Agent Operations Center
          </h1>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowRegister(true)}
            className="flex items-center gap-2 rounded-lg bg-black px-3 py-1.5 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-black dark:hover:bg-gray-200"
          >
            + Connect Agent
          </button>
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
          <div className="grid gap-4 md:grid-cols-4 xl:grid-cols-5">
            <SummaryCard icon={<Bot className="h-4 w-4" />} label="Connected Agents" value={String(connectedAgents.length)} />
            <SummaryCard icon={<Activity className="h-4 w-4" />} label="Busy/Online" value={String(connectedAgents.filter((a) => ['busy', 'online'].includes((a.agent?.status || '').toLowerCase())).length)} />
            <SummaryCard icon={<Bell className="h-4 w-4" />} label="Blocked Agents" value={String(connectedAgents.filter((a) => (a.agent?.status || '').toLowerCase() === 'blocked').length)} />
            <SummaryCard icon={<Clock3 className="h-4 w-4" />} label="Assigned Tasks" value={String((tasks || []).filter((t) => !!t.assignee_id && t.status !== 'done').length)} />
            <SummaryCard icon={<PlugZap className="h-4 w-4" />} label="Clawn Native" value={String(clawnAgents.length)} />
          </div>

          <div className="rounded-2xl border bg-card p-4 shadow-sm">
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <div className="font-semibold text-foreground">Quick paths:</div>
              <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">External → webhook / REST</span>
              <span className="rounded-full bg-rose-100 px-3 py-1 text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">Clawn Native → OpenClaw / PicoClaw</span>
              <span className="rounded-full bg-green-100 px-3 py-1 text-green-700 dark:bg-green-900/20 dark:text-green-300">Machine token runtime auth</span>
            </div>
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            {connectedAgents.map((entry) => (
              <AgentCard key={`${entry.board_id}-${entry.agent_id}`} boardAgent={entry} tasks={tasks || []} />
            ))}
          </div>

          {connectedAgents.length === 0 && (
            <div className="rounded-xl border border-dashed p-10 text-center text-sm text-muted-foreground">
              Belum ada agent yang terhubung ke board ini. Klik <strong>+ Connect Agent</strong> buat mulai.
            </div>
          )}
        </div>
      </div>

      <MemberManager boardId={id} isOpen={showMembers} onClose={() => setShowMembers(false)} />

      <RegisterAgentModal
        boardId={id}
        isOpen={showRegister}
        onClose={() => setShowRegister(false)}
        onSuccess={() => {
          mutateBoardAgents();
        }}
      />
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

function AgentCard({ boardAgent, tasks }: { boardAgent: BoardAgent; tasks: Task[] }) {
  const agent = boardAgent.agent;
  const { data: runs } = useSWR<AgentRun[]>(agent ? `/api/agents/${agent.id}/runs` : null, fetcher, { refreshInterval: 10000 });
  const { data: notifications } = useSWR<Notification[]>(agent ? `/api/agents/${agent.id}/notifications` : null, fetcher, { refreshInterval: 10000 });

  if (!agent) return null;

  const currentTask = tasks.find((task) => task.id === agent.current_task_id || task.assignee_id === agent.id);
  const pendingNotifications = (notifications || []).filter((n) => !n.delivered);
  const latestRun = runs?.[0];

  return (
    <div className="rounded-2xl border bg-card p-5 shadow-sm">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-rose-100 text-lg dark:bg-rose-900/30">{agent.avatar || '🤖'}</div>
            <div>
              <div className="flex items-center gap-2">
                <div className="text-lg font-semibold">{agent.display_name || agent.id}</div>
                {agent.is_native_clawn && (
                  <span className="rounded-full bg-rose-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">
                    Clawn Native
                  </span>
                )}
              </div>
              <div className="text-xs text-muted-foreground">{agent.id} · {agent.provider || 'external'} / {agent.engine || 'custom'}</div>
            </div>
          </div>
        </div>
        <StatusBadge status={agent.status} />
      </div>

      <div className="mb-4 grid gap-3 md:grid-cols-2">
        <InfoRow label="Board Role" value={boardAgent.board_role} />
        <InfoRow label="Mode" value={boardAgent.auto_accept_tasks ? 'Auto-accept enabled' : 'Manual accept'} />
        <InfoRow label="Current Task" value={currentTask ? `${currentTask.title} (#${currentTask.id})` : 'Idle'} />
        <InfoRow label="Current Activity" value={agent.current_activity || latestRun?.current_activity || '-'} />
        <InfoRow label="Last Heartbeat" value={agent.last_heartbeat_at ? new Date(agent.last_heartbeat_at).toLocaleString() : '-'} />
        <InfoRow label="Health Note" value={agent.health_note || latestRun?.error_summary || '-'} />
      </div>

      {agent.description && (
        <div className="mb-4 rounded-xl border bg-muted/20 p-3 text-sm text-muted-foreground">
          {agent.description}
        </div>
      )}

      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <div className="rounded-xl bg-muted/40 p-4">
          <div className="mb-2 flex items-center gap-2 text-sm font-semibold"><PlayCircle className="h-4 w-4" /> Latest Run</div>
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
          <div className="mb-2 flex items-center gap-2 text-sm font-semibold"><Bell className="h-4 w-4" /> Pending Signals</div>
          {pendingNotifications.length > 0 ? (
            <div className="space-y-2">
              {pendingNotifications.slice(0, 3).map((notif) => (
                <div key={notif.id} className="rounded-lg border bg-background p-2 text-sm">
                  <div className="font-medium">{notif.type.replace(/_/g, ' ')}</div>
                  <div className="text-xs text-muted-foreground">{notif.content}</div>
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
          <AlertTriangle className="mt-0.5 h-4 w-4" />
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
