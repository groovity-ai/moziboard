'use client';

import React, { useMemo, useState } from 'react';
import useSWR, { mutate } from 'swr';
import Link from 'next/link';
import {
    Plus,
    LayoutDashboard,
    ChevronRight,
    Loader2,
    Sparkles,
    ArrowRight,
    FolderKanban,
    CheckCircle2,
    Clock3,
    AlertTriangle,
    Bot,
    FileText,
    Activity,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';

const fetcher = async (url: string) => {
    const res = await fetch(url, {
        credentials: 'include',
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Request failed: ${res.status}`);
    }
    return res.json();
};

type BoardOverview = {
    id: string;
    title: string;
    description?: string;
    health: 'healthy' | 'needs_review' | 'blocked' | 'agent_issue' | 'quiet' | string;
    last_activity_at?: string | null;
    stats_computed_at?: string | null;
    stats_age_seconds?: number;
    is_stale?: boolean;
    metrics: {
        open_tasks: number;
        review_tasks: number;
        blocked_tasks: number;
        docs_count: number;
        connected_agents: number;
        agent_issues: number;
    };
};

type AttentionItem = {
    type: string;
    board_id: string;
    board_title: string;
    task_id?: number | null;
    task_title?: string;
    agent_id?: string;
    agent_name?: string;
    title: string;
    description?: string;
    created_at?: string | null;
    href?: string;
};

type ActivityItem = {
    type: string;
    board_id: string;
    board_title: string;
    task_id?: number | null;
    task_title?: string;
    title: string;
    created_at?: string | null;
    href?: string;
};

type OverviewPayload = {
    summary: {
        boards: number;
        needs_attention: number;
        blocked_tasks: number;
        pending_reviews: number;
    };
    boards: BoardOverview[];
    attention: AttentionItem[];
    recent_activity: ActivityItem[];
    cache?: {
        version: number;
        cached_at?: string | null;
        expires_at?: string | null;
        from_cache: boolean;
    };
};

function timeAgo(value?: string | null) {
    if (!value) return 'No recent activity';
    const ts = new Date(value).getTime();
    if (Number.isNaN(ts)) return 'No recent activity';
    const diffMs = Date.now() - ts;
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'Just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    const diffDay = Math.floor(diffHr / 24);
    return `${diffDay}d ago`;
}

function healthTone(health?: string) {
    switch ((health || '').toLowerCase()) {
        case 'blocked':
            return 'bg-red-500/10 text-red-600 dark:text-red-300';
        case 'needs_review':
            return 'bg-amber-500/10 text-amber-700 dark:text-amber-300';
        case 'agent_issue':
            return 'bg-violet-500/10 text-violet-700 dark:text-violet-300';
        case 'healthy':
            return 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-300';
        default:
            return 'bg-zinc-500/10 text-zinc-600 dark:text-zinc-300';
    }
}

function healthLabel(health?: string) {
    switch ((health || '').toLowerCase()) {
        case 'blocked':
            return 'Blocked';
        case 'needs_review':
            return 'Needs review';
        case 'agent_issue':
            return 'Agent issue';
        case 'healthy':
            return 'Healthy';
        default:
            return 'Quiet';
    }
}

function attentionIcon(type: string) {
    if (type === 'blocked_task') return <AlertTriangle className="h-4 w-4 text-red-500" />;
    if (type === 'review_task') return <CheckCircle2 className="h-4 w-4 text-amber-500" />;
    return <Bot className="h-4 w-4 text-violet-500" />;
}

function activityIcon(type: string) {
    if (type === 'document') return <FileText className="h-4 w-4 text-emerald-500" />;
    if (type === 'agent_event_failure') return <AlertTriangle className="h-4 w-4 text-red-500" />;
    return <Activity className="h-4 w-4 text-rose-500" />;
}

function getBoardTheme(board: BoardOverview) {
    const health = (board.health || '').toLowerCase();
    if (health === 'blocked') {
        return {
            card: 'hover:border-red-500/30 hover:shadow-red-500/10',
            glow: 'from-red-500/14 via-red-500/6 to-transparent',
            divider: 'from-red-500/20 via-border/60 to-transparent',
            avatar: 'from-red-500/20 to-red-500/5 text-red-600 ring-red-500/15 dark:text-red-300',
            cta: 'text-red-500',
            button: 'group-hover:border-red-500/20 group-hover:text-red-500',
            insight: 'bg-red-500/10 text-red-600 dark:text-red-300',
        };
    }
    if (health === 'needs_review') {
        return {
            card: 'hover:border-amber-500/30 hover:shadow-amber-500/10',
            glow: 'from-amber-500/14 via-amber-500/6 to-transparent',
            divider: 'from-amber-500/20 via-border/60 to-transparent',
            avatar: 'from-amber-500/20 to-amber-500/5 text-amber-700 ring-amber-500/15 dark:text-amber-300',
            cta: 'text-amber-600 dark:text-amber-300',
            button: 'group-hover:border-amber-500/20 group-hover:text-amber-600 dark:group-hover:text-amber-300',
            insight: 'bg-amber-500/10 text-amber-700 dark:text-amber-300',
        };
    }
    if (health === 'agent_issue') {
        return {
            card: 'hover:border-violet-500/30 hover:shadow-violet-500/10',
            glow: 'from-violet-500/14 via-violet-500/6 to-transparent',
            divider: 'from-violet-500/20 via-border/60 to-transparent',
            avatar: 'from-violet-500/20 to-violet-500/5 text-violet-700 ring-violet-500/15 dark:text-violet-300',
            cta: 'text-violet-600 dark:text-violet-300',
            button: 'group-hover:border-violet-500/20 group-hover:text-violet-600 dark:group-hover:text-violet-300',
            insight: 'bg-violet-500/10 text-violet-700 dark:text-violet-300',
        };
    }
    if (health === 'quiet') {
        return {
            card: 'hover:border-zinc-400/30 hover:shadow-zinc-500/10',
            glow: 'from-zinc-500/10 via-zinc-500/4 to-transparent',
            divider: 'from-zinc-500/15 via-border/60 to-transparent',
            avatar: 'from-zinc-500/15 to-zinc-500/5 text-zinc-700 ring-zinc-500/10 dark:text-zinc-300',
            cta: 'text-zinc-600 dark:text-zinc-300',
            button: 'group-hover:border-zinc-400/20 group-hover:text-zinc-600 dark:group-hover:text-zinc-300',
            insight: 'bg-zinc-500/10 text-zinc-700 dark:text-zinc-300',
        };
    }
    return {
        card: 'hover:border-emerald-500/30 hover:shadow-emerald-500/10',
        glow: 'from-emerald-500/14 via-rose-500/5 to-transparent',
        divider: 'from-emerald-500/20 via-border/60 to-transparent',
        avatar: 'from-emerald-500/20 to-emerald-500/5 text-emerald-700 ring-emerald-500/15 dark:text-emerald-300',
        cta: 'text-emerald-600 dark:text-emerald-300',
        button: 'group-hover:border-emerald-500/20 group-hover:text-emerald-600 dark:group-hover:text-emerald-300',
        insight: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
    };
}

function getBoardHighlight(board: BoardOverview) {
    if (board.metrics.blocked_tasks > 0) {
        return {
            title: `${board.metrics.blocked_tasks} blocker active`,
            detail: 'Ada task yang lagi nahan progress board ini.',
        };
    }
    if (board.metrics.review_tasks > 0) {
        return {
            title: `${board.metrics.review_tasks} task waiting review`,
            detail: 'Board ini lagi numpuk pekerjaan yang perlu decision atau QA.',
        };
    }
    if (board.metrics.agent_issues > 0) {
        return {
            title: `${board.metrics.agent_issues} agent needs attention`,
            detail: 'Ada workforce/runtime issue yang layak dicek dulu.',
        };
    }
    if (board.metrics.connected_agents > 0) {
        return {
            title: `${board.metrics.connected_agents} agents connected`,
            detail: 'Board ini udah punya workforce aktif yang siap jalan.',
        };
    }
    if (board.metrics.docs_count >= 3) {
        return {
            title: `${board.metrics.docs_count} docs in place`,
            detail: 'Knowledge base board ini udah lumayan kebentuk.',
        };
    }
    if (board.metrics.open_tasks > 0) {
        return {
            title: `${board.metrics.open_tasks} tasks in motion`,
            detail: 'Masih ada pekerjaan aktif yang bisa langsung dilanjut.',
        };
    }
    return {
        title: 'Quiet workspace',
        detail: 'Board ini masih tenang dan cocok buat mulai konteks baru.',
    };
}

function getBoardCTA(board: BoardOverview) {
    const health = (board.health || '').toLowerCase();
    if (health === 'blocked') return { label: 'Resolve blockers', href: `/board/${board.id}` };
    if (health === 'needs_review') return { label: 'Open reviews', href: `/board/${board.id}` };
    if (health === 'agent_issue') return { label: 'Check ops', href: `/board/${board.id}/ops` };
    if (board.metrics.docs_count >= 3 && board.metrics.open_tasks === 0) return { label: 'Open docs', href: `/board/${board.id}/docs` };
    return { label: 'Continue work', href: `/board/${board.id}` };
}

function getBoardVariant(board: BoardOverview) {
    if (board.metrics.blocked_tasks > 0) return 'alert';
    if (board.metrics.review_tasks > 0) return 'review';
    if (board.metrics.agent_issues > 0 || board.metrics.connected_agents > 0) return 'runtime';
    if (board.metrics.docs_count >= 3) return 'knowledge';
    return 'default';
}

function getPrimaryMetric(board: BoardOverview) {
    if (board.metrics.blocked_tasks > 0) {
        return {
            label: 'Primary focus',
            value: `${board.metrics.blocked_tasks}`,
            suffix: 'blocked',
            tone: 'bg-red-500/10 text-red-600 dark:text-red-300',
        };
    }
    if (board.metrics.review_tasks > 0) {
        return {
            label: 'Primary focus',
            value: `${board.metrics.review_tasks}`,
            suffix: 'in review',
            tone: 'bg-amber-500/10 text-amber-700 dark:text-amber-300',
        };
    }
    if (board.metrics.connected_agents > 0) {
        return {
            label: 'Primary focus',
            value: `${board.metrics.connected_agents}`,
            suffix: 'agents active',
            tone: 'bg-violet-500/10 text-violet-700 dark:text-violet-300',
        };
    }
    if (board.metrics.docs_count > 0) {
        return {
            label: 'Primary focus',
            value: `${board.metrics.docs_count}`,
            suffix: 'docs ready',
            tone: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
        };
    }
    return {
        label: 'Primary focus',
        value: `${board.metrics.open_tasks}`,
        suffix: 'open tasks',
        tone: 'bg-zinc-500/10 text-zinc-700 dark:text-zinc-300',
    };
}

function BoardCard({ board }: { board: BoardOverview }) {
    const initial = (board.title || 'B').trim().charAt(0).toUpperCase();
    const hasDescription = Boolean(board.description?.trim());
    const theme = getBoardTheme(board);
    const highlight = getBoardHighlight(board);
    const cta = getBoardCTA(board);
    const variant = getBoardVariant(board);
    const primaryMetric = getPrimaryMetric(board);

    const patternClass =
        variant === 'alert'
            ? 'bg-[linear-gradient(135deg,rgba(239,68,68,0.08)_0%,transparent_35%,rgba(239,68,68,0.04)_100%)]'
            : variant === 'review'
                ? 'bg-[linear-gradient(135deg,rgba(245,158,11,0.08)_0%,transparent_35%,rgba(245,158,11,0.04)_100%)]'
                : variant === 'runtime'
                    ? 'bg-[linear-gradient(135deg,rgba(139,92,246,0.08)_0%,transparent_35%,rgba(139,92,246,0.04)_100%)]'
                    : variant === 'knowledge'
                        ? 'bg-[linear-gradient(135deg,rgba(16,185,129,0.08)_0%,transparent_35%,rgba(16,185,129,0.04)_100%)]'
                        : 'bg-[linear-gradient(135deg,rgba(244,63,94,0.06)_0%,transparent_35%,rgba(244,63,94,0.03)_100%)]';

    return (
        <div className={`group relative flex h-full min-h-[334px] flex-col overflow-hidden rounded-[30px] border border-border/60 bg-card/95 p-6 shadow-sm transition-all duration-200 hover:-translate-y-1.5 hover:shadow-xl ${theme.card}`}>
            <div className={`absolute inset-x-0 top-0 h-28 bg-gradient-to-br ${theme.glow} opacity-90`} />
            <div className={`absolute inset-0 opacity-70 ${patternClass}`} />
            <div className="absolute right-5 top-5 h-24 w-24 rounded-full border border-white/5 bg-white/[0.03] blur-2xl" />
            <div className={`absolute inset-x-6 top-[88px] h-px bg-gradient-to-r ${theme.divider}`} />

            <div className="relative flex h-full flex-col">
                <div className="flex items-start justify-between gap-4">
                    <div className="flex min-w-0 items-center gap-3">
                        <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br text-sm font-bold ring-1 ${theme.avatar}`}>
                            {initial}
                        </div>
                        <div className="min-w-0">
                            <Link href={`/board/${board.id}`} className="truncate text-lg font-semibold tracking-tight transition-colors group-hover:text-foreground block">
                                {board.title}
                            </Link>
                            <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
                                <span className="rounded-full border border-border/60 bg-background/80 px-2.5 py-1 font-medium">Workspace</span>
                                <span className={`rounded-full px-2.5 py-1 font-medium ${healthTone(board.health)}`}>{healthLabel(board.health)}</span>
                                {!hasDescription && (
                                    <span className="rounded-full bg-amber-500/10 px-2.5 py-1 font-medium text-amber-700 dark:text-amber-300">Needs context</span>
                                )}
                            </div>
                        </div>
                    </div>

                    <Link href={cta.href} className={`rounded-full border border-border/60 bg-background/90 p-2 text-muted-foreground opacity-0 transition-all duration-200 group-hover:translate-x-0.5 group-hover:opacity-100 ${theme.button}`}>
                        <ChevronRight size={16} />
                    </Link>
                </div>

                <div className="mt-5 grid grid-cols-[1.1fr_0.9fr] gap-3">
                    <div className="rounded-2xl border border-border/60 bg-background/75 p-4">
                        <div className={`inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-medium ${theme.insight}`}>
                            Highlight
                        </div>
                        <div className="mt-3 text-sm font-semibold tracking-tight">{highlight.title}</div>
                        <p className="mt-1.5 line-clamp-2 text-xs leading-5 text-muted-foreground">{highlight.detail}</p>
                    </div>
                    <div className="rounded-2xl border border-border/60 bg-background/75 p-4">
                        <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">{primaryMetric.label}</div>
                        <div className="mt-3 flex items-end gap-2">
                            <div className="text-3xl font-semibold tracking-tight">{primaryMetric.value}</div>
                            <div className={`mb-1 rounded-full px-2 py-1 text-[10px] font-medium ${primaryMetric.tone}`}>{primaryMetric.suffix}</div>
                        </div>
                    </div>
                </div>

                <div className="relative mt-5 flex-1">
                    <p className="line-clamp-3 text-sm leading-6 text-muted-foreground">
                        {board.description?.trim() || 'Belum ada deskripsi. Buka board ini buat mulai ngatur task, docs, member, dan workflow agent dalam satu workspace.'}
                    </p>
                </div>

                <div className={`relative mt-6 grid gap-3 ${variant === 'knowledge' ? 'grid-cols-1' : 'grid-cols-2'}`}>
                    <div className="rounded-2xl border border-border/60 bg-background/80 px-3 py-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
                        <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">Tasks</div>
                        <div className="mt-1.5 text-sm font-medium">{board.metrics.open_tasks} open • {board.metrics.review_tasks} review</div>
                    </div>
                    <div className="rounded-2xl border border-border/60 bg-background/80 px-3 py-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
                        <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">
                            {variant === 'runtime' ? 'Workforce' : variant === 'knowledge' ? 'Knowledge base' : 'Resources'}
                        </div>
                        <div className="mt-1.5 text-sm font-medium">
                            {variant === 'runtime'
                                ? `${board.metrics.connected_agents} agents • ${board.metrics.agent_issues} issues`
                                : variant === 'knowledge'
                                    ? `${board.metrics.docs_count} docs ready • ${timeAgo(board.last_activity_at)}`
                                    : `${board.metrics.docs_count} docs • ${board.metrics.connected_agents} agents`}
                        </div>
                    </div>
                </div>

                <div className="mt-4 flex flex-wrap gap-2 text-xs">
                    {board.metrics.blocked_tasks > 0 && (
                        <span className="rounded-full bg-red-500/10 px-2.5 py-1 font-medium text-red-600 dark:text-red-300">
                            {board.metrics.blocked_tasks} blocked
                        </span>
                    )}
                    {board.metrics.agent_issues > 0 && (
                        <span className="rounded-full bg-violet-500/10 px-2.5 py-1 font-medium text-violet-700 dark:text-violet-300">
                            {board.metrics.agent_issues} agent issues
                        </span>
                    )}
                    {board.is_stale ? (
                        <span className="rounded-full bg-amber-500/10 px-2.5 py-1 font-medium text-amber-700 dark:text-amber-300">
                            Stats stale · {board.stats_age_seconds || 0}s old
                        </span>
                    ) : board.stats_computed_at ? (
                        <span className="rounded-full bg-emerald-500/10 px-2.5 py-1 font-medium text-emerald-700 dark:text-emerald-300">
                            Stats fresh · {timeAgo(board.stats_computed_at)}
                        </span>
                    ) : null}
                    <span className="rounded-full border border-border/60 bg-background/70 px-2.5 py-1 font-medium text-muted-foreground">
                        {timeAgo(board.last_activity_at)}
                    </span>
                </div>

                <div className="relative mt-5 flex items-center justify-between border-t border-border/50 pt-4 text-sm">
                    <div className="flex flex-wrap items-center gap-3 text-muted-foreground">
                        <Link href={`/board/${board.id}`} className="transition hover:text-foreground">Board</Link>
                        <Link href={`/board/${board.id}/docs`} className="transition hover:text-foreground">Docs</Link>
                        <Link href={`/board/${board.id}/ops`} className="transition hover:text-foreground">Ops</Link>
                    </div>
                    <Link href={cta.href} className={`inline-flex items-center gap-1.5 font-medium ${theme.cta}`}>
                        {cta.label}
                        <ChevronRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
                    </Link>
                </div>
            </div>
        </div>
    );
}

export default function Dashboard() {
    const { data, error, isLoading } = useSWR<OverviewPayload>('/api/home/overview', fetcher, {
        revalidateOnFocus: false,
    });
    const [isCreating, setIsCreating] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [newTitle, setNewTitle] = useState('');
    const [newDesc, setNewDesc] = useState('');

    const boardCount = data?.summary?.boards || 0;
    const needsAttentionCount = data?.summary?.needs_attention || 0;
    const blockedTasks = data?.summary?.blocked_tasks || 0;
    const pendingReviews = data?.summary?.pending_reviews || 0;
    const boards = data?.boards || [];
    const attention = data?.attention || [];
    const recentActivity = data?.recent_activity || [];
    const cacheMeta = data?.cache;

    const describedBoards = useMemo(() => boards.filter((board) => board.description?.trim()).length, [boards]);
    const staleBoards = useMemo(() => boards.filter((board) => board.is_stale).length, [boards]);

    const handleCreate = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsSubmitting(true);
        try {
            const res = await fetch('/api/boards', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({ title: newTitle, description: newDesc }),
            });

            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || 'Failed to create board');
            }

            await Promise.all([
                mutate('/api/boards'),
                mutate('/api/home/overview'),
            ]);
            setIsCreating(false);
            setNewTitle('');
            setNewDesc('');
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <div className="min-h-screen bg-gradient-to-br from-background via-background to-rose-500/5">
            <header className="sticky top-0 z-50 border-b bg-background/90 backdrop-blur supports-[backdrop-filter]:bg-background/75">
                <div className="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6">
                    <div className="flex min-w-0 items-center gap-3">
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-rose-500 text-white shadow-sm shadow-rose-500/20">
                            <LayoutDashboard size={18} />
                        </div>
                        <div className="min-w-0">
                            <div className="truncate text-sm font-semibold tracking-tight sm:text-base">ClawnBoard</div>
                            <div className="text-[11px] text-muted-foreground sm:text-xs">Boards, docs, and agent workspaces in one place</div>
                        </div>
                    </div>

                    <Button onClick={() => setIsCreating(true)} size="sm" className="rounded-xl">
                        <Plus className="mr-2 h-4 w-4" />
                        New Board
                    </Button>
                </div>
            </header>

            <main className="mx-auto flex w-full max-w-7xl flex-col gap-8 px-4 py-6 sm:px-6 sm:py-8">
                <section className="overflow-hidden rounded-[32px] border border-border/60 bg-card/80 shadow-sm">
                    <div className="relative px-6 py-8 sm:px-8 sm:py-10">
                        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(244,63,94,0.12),transparent_35%),radial-gradient(circle_at_right,rgba(244,63,94,0.08),transparent_30%)]" />
                        <div className="absolute inset-x-0 bottom-0 h-px bg-gradient-to-r from-transparent via-border/70 to-transparent" />

                        <div className="relative flex flex-col gap-8 lg:flex-row lg:items-end lg:justify-between">
                            <div className="max-w-2xl">
                                <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-rose-500/15 bg-rose-500/10 px-3 py-1 text-xs font-medium text-rose-600 dark:text-rose-300">
                                    <Sparkles className="h-3.5 w-3.5" />
                                    ClawnBoard Hub
                                </div>
                                <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
                                    Semua workspace lu, rapi dalam satu hub yang enak buat lanjut kerja.
                                </h1>
                                <p className="mt-4 max-w-xl text-sm leading-6 text-muted-foreground sm:text-base">
                                    Masuk ke board, buka knowledge base, dan lanjutkan eksekusi tim maupun agent dari permukaan yang konsisten.
                                </p>
                                <div className="mt-6 flex flex-wrap items-center gap-3">
                                    <Button onClick={() => setIsCreating(true)} className="rounded-xl shadow-sm shadow-rose-500/10">
                                        <Plus className="mr-2 h-4 w-4" />
                                        Create board
                                    </Button>
                                    {boardCount > 0 && (
                                        <div className="inline-flex items-center gap-2 rounded-xl border bg-background/80 px-3 py-2 text-sm text-muted-foreground shadow-sm">
                                            <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                                            {boardCount} workspace siap lu lanjutkan
                                        </div>
                                    )}
                                    <div className="inline-flex items-center gap-2 rounded-xl border border-border/60 bg-background/70 px-3 py-2 text-sm text-muted-foreground">
                                        <span className="inline-block h-2 w-2 rounded-full bg-rose-500" />
                                        Board list sekarang jadi pintu masuk utama workspace lu
                                    </div>
                                    {cacheMeta && (
                                        <div className="inline-flex items-center gap-2 rounded-xl border border-border/60 bg-background/70 px-3 py-2 text-sm text-muted-foreground">
                                            <span className={`inline-block h-2 w-2 rounded-full ${cacheMeta.from_cache ? 'bg-amber-500' : 'bg-emerald-500'}`} />
                                            {cacheMeta.from_cache ? `Cached ${timeAgo(cacheMeta.cached_at)}` : 'Freshly computed'} · v{cacheMeta.version}
                                        </div>
                                    )}
                                    {staleBoards > 0 && (
                                        <div className="inline-flex items-center gap-2 rounded-xl border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                                            <AlertTriangle className="h-4 w-4" />
                                            {staleBoards} board stats stale
                                        </div>
                                    )}
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-3 lg:min-w-[460px] lg:max-w-[500px] lg:flex-1">
                                <div className="rounded-2xl border border-border/60 bg-background/80 p-4">
                                    <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                        <FolderKanban className="h-3.5 w-3.5" /> Boards
                                    </div>
                                    <div className="mt-3 text-3xl font-semibold tracking-tight">{boardCount}</div>
                                    <p className="mt-1 text-xs text-muted-foreground">Total board yang bisa lu buka sekarang</p>
                                </div>
                                <div className="rounded-2xl border border-border/60 bg-background/80 p-4">
                                    <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                        <AlertTriangle className="h-3.5 w-3.5" /> Attention
                                    </div>
                                    <div className="mt-3 text-3xl font-semibold tracking-tight">{needsAttentionCount}</div>
                                    <p className="mt-1 text-xs text-muted-foreground">Board yang lagi butuh perhatian</p>
                                </div>
                                <div className="rounded-2xl border border-border/60 bg-background/80 p-4">
                                    <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                        <Clock3 className="h-3.5 w-3.5" /> Blocked
                                    </div>
                                    <div className="mt-3 text-3xl font-semibold tracking-tight">{blockedTasks}</div>
                                    <p className="mt-1 text-xs text-muted-foreground">Task yang lagi ketahan</p>
                                </div>
                                <div className="rounded-2xl border border-border/60 bg-background/80 p-4">
                                    <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                        <CheckCircle2 className="h-3.5 w-3.5" /> Review
                                    </div>
                                    <div className="mt-3 text-3xl font-semibold tracking-tight">{pendingReviews}</div>
                                    <p className="mt-1 text-xs text-muted-foreground">Task yang nunggu review</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </section>

                {!isLoading && !error && boardCount > 0 && (
                    <section className="grid grid-cols-1 gap-5 xl:grid-cols-[1.1fr_0.9fr]">
                        <div className="rounded-[28px] border border-border/60 bg-card/80 p-5 shadow-sm">
                            <div className="mb-4 flex items-center justify-between gap-3">
                                <div>
                                    <div className="text-sm font-semibold tracking-tight">Needs attention</div>
                                    <p className="mt-1 text-xs text-muted-foreground">Yang paling butuh lu lihat duluan sekarang.</p>
                                </div>
                                <div className="rounded-full bg-rose-500/10 px-2.5 py-1 text-xs font-medium text-rose-600 dark:text-rose-300">
                                    {attention.length} items
                                </div>
                            </div>

                            {attention.length === 0 ? (
                                <div className="rounded-2xl border border-dashed border-border bg-background/60 px-4 py-8 text-sm text-muted-foreground">
                                    Belum ada alert penting. Homepage lagi cukup sehat.
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {attention.map((item, index) => (
                                        <Link
                                            key={`${item.type}-${item.board_id}-${item.task_id || item.agent_id || index}`}
                                            href={item.href || `/board/${item.board_id}`}
                                            className="flex items-start gap-3 rounded-2xl border border-border/60 bg-background/70 px-4 py-3 transition hover:border-rose-500/20 hover:bg-background"
                                        >
                                            <div className="mt-0.5">{attentionIcon(item.type)}</div>
                                            <div className="min-w-0 flex-1">
                                                <div className="truncate text-sm font-medium">{item.title}</div>
                                                <div className="mt-1 text-xs text-muted-foreground">{item.board_title}</div>
                                                {item.description ? (
                                                    <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{item.description}</div>
                                                ) : null}
                                            </div>
                                            <div className="shrink-0 text-[11px] text-muted-foreground">{timeAgo(item.created_at)}</div>
                                        </Link>
                                    ))}
                                </div>
                            )}
                        </div>

                        <div className="rounded-[28px] border border-border/60 bg-card/80 p-5 shadow-sm">
                            <div className="mb-4 flex items-center justify-between gap-3">
                                <div>
                                    <div className="text-sm font-semibold tracking-tight">Recent activity</div>
                                    <p className="mt-1 text-xs text-muted-foreground">Pergerakan terbaru lintas board.</p>
                                </div>
                                <div className="rounded-full bg-background/80 px-2.5 py-1 text-xs font-medium text-muted-foreground">
                                    {recentActivity.length} events
                                </div>
                            </div>

                            {recentActivity.length === 0 ? (
                                <div className="rounded-2xl border border-dashed border-border bg-background/60 px-4 py-8 text-sm text-muted-foreground">
                                    Belum ada aktivitas terbaru yang signifikan.
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {recentActivity.map((item, index) => (
                                        <Link
                                            key={`${item.type}-${item.board_id}-${item.task_id || index}`}
                                            href={item.href || `/board/${item.board_id}`}
                                            className="flex items-start gap-3 rounded-2xl border border-border/60 bg-background/70 px-4 py-3 transition hover:border-rose-500/20 hover:bg-background"
                                        >
                                            <div className="mt-0.5">{activityIcon(item.type)}</div>
                                            <div className="min-w-0 flex-1">
                                                <div className="line-clamp-2 text-sm font-medium">{item.title}</div>
                                                <div className="mt-1 text-xs text-muted-foreground">{item.board_title}</div>
                                            </div>
                                            <div className="shrink-0 text-[11px] text-muted-foreground">{timeAgo(item.created_at)}</div>
                                        </Link>
                                    ))}
                                </div>
                            )}
                        </div>
                    </section>
                )}

                {isLoading ? (
                    <section className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3">
                        {Array.from({ length: 6 }).map((_, index) => (
                            <div key={index} className="min-h-[240px] animate-pulse rounded-[28px] border border-border/60 bg-card/70 p-6">
                                <div className="h-12 w-12 rounded-2xl bg-muted" />
                                <div className="mt-5 h-5 w-2/3 rounded bg-muted" />
                                <div className="mt-3 h-4 w-full rounded bg-muted" />
                                <div className="mt-2 h-4 w-5/6 rounded bg-muted" />
                                <div className="mt-8 grid grid-cols-2 gap-3">
                                    <div className="h-16 rounded-2xl bg-muted" />
                                    <div className="h-16 rounded-2xl bg-muted" />
                                </div>
                            </div>
                        ))}
                    </section>
                ) : error ? (
                    <section className="rounded-[32px] border border-dashed border-border bg-card/60 px-6 py-16 text-center shadow-sm">
                        <div className="mx-auto max-w-xl">
                            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-rose-500/10 text-rose-500">
                                <LayoutDashboard className="h-6 w-6" />
                            </div>
                            <h2 className="mt-5 text-2xl font-semibold tracking-tight">Workspace list belum bisa dimuat</h2>
                            <p className="mt-3 text-sm leading-6 text-muted-foreground">
                                Kemungkinan masih ada issue auth atau session di account ini. Kalau perlu, kita cek endpoint `/api/home/overview` atau flow login-nya biar dashboard balik normal.
                            </p>
                            <div className="mt-6">
                                <Button variant="outline" onClick={() => mutate('/api/home/overview')} className="rounded-xl">
                                    Coba refresh data
                                </Button>
                            </div>
                        </div>
                    </section>
                ) : boardCount === 0 ? (
                    <section className="rounded-[32px] border border-dashed border-border bg-card/60 px-6 py-16 text-center shadow-sm">
                        <div className="mx-auto max-w-2xl">
                            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-3xl bg-rose-500/10 text-rose-500">
                                <Sparkles className="h-7 w-7" />
                            </div>
                            <h2 className="mt-5 text-3xl font-semibold tracking-tight">Belum ada workspace yang bisa lu akses</h2>
                            <p className="mt-4 text-sm leading-6 text-muted-foreground sm:text-base">
                                Bisa jadi ini account baru, atau membership board belum ke-assign. Bikin board pertama dulu biar workspace kebuka dan flow ClawnBoard bisa dites end-to-end.
                            </p>
                            <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
                                <Button onClick={() => setIsCreating(true)} disabled={isSubmitting} className="rounded-xl">
                                    {isSubmitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
                                    Create first board
                                </Button>
                            </div>
                        </div>
                    </section>
                ) : (
                    <section className="space-y-5">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                            <div>
                                <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-border/60 bg-background/70 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
                                    Workspace directory
                                </div>
                                <h2 className="text-2xl font-semibold tracking-tight">Your workspaces</h2>
                                <p className="mt-1 text-sm text-muted-foreground">
                                    Pilih workspace buat lanjut ke kanban, docs, atau operasional agent yang lagi jalan.
                                </p>
                            </div>
                            <Button variant="outline" onClick={() => setIsCreating(true)} className="rounded-xl bg-background/80">
                                <Plus className="mr-2 h-4 w-4" /> Add another board
                            </Button>
                        </div>

                        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3">
                            <button
                                onClick={() => setIsCreating(true)}
                                className="group relative flex min-h-[248px] flex-col justify-between overflow-hidden rounded-[30px] border border-dashed border-border bg-card/55 p-6 text-left transition-all duration-200 hover:-translate-y-1.5 hover:border-rose-500/30 hover:bg-rose-500/5 hover:shadow-lg hover:shadow-rose-500/5"
                            >
                                <div className="absolute inset-x-0 top-0 h-24 bg-gradient-to-br from-rose-500/10 via-rose-500/5 to-transparent opacity-80" />
                                <div className="relative">
                                    <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-rose-500/10 text-rose-500 transition-colors group-hover:bg-rose-500 group-hover:text-white">
                                        <Plus size={22} />
                                    </div>
                                    <div className="mt-4 inline-flex items-center rounded-full border border-border/60 bg-background/80 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground">
                                        New workspace
                                    </div>
                                    <h3 className="mt-5 text-xl font-semibold tracking-tight">Create a new board</h3>
                                    <p className="mt-3 text-sm leading-6 text-muted-foreground">
                                        Mulai workspace baru buat product, sprint, client work, atau board operasional agent yang lebih spesifik.
                                    </p>
                                </div>

                                <div className="relative mt-6 inline-flex items-center gap-2 text-sm font-medium text-rose-500">
                                    Create workspace
                                    <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-1" />
                                </div>
                            </button>

                            {boards.map((board) => (
                                <BoardCard key={board.id} board={board} />
                            ))}
                        </div>
                    </section>
                )}
            </main>

            <Dialog open={isCreating} onOpenChange={setIsCreating}>
                <DialogContent className="sm:max-w-[460px]">
                    <form onSubmit={handleCreate}>
                        <DialogHeader>
                            <DialogTitle>Create New Board</DialogTitle>
                            <DialogDescription>
                                Bikin workspace baru buat ngatur task, docs, tim, dan connected agents dalam satu tempat.
                            </DialogDescription>
                        </DialogHeader>
                        <div className="grid gap-4 py-4">
                            <div className="grid gap-2">
                                <Label htmlFor="title">Board title</Label>
                                <Input
                                    id="title"
                                    placeholder="e.g. Clawn Growth Sprint"
                                    value={newTitle}
                                    onChange={(e) => setNewTitle(e.target.value)}
                                    required
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="description">Description</Label>
                                <Input
                                    id="description"
                                    placeholder="Short context for this workspace..."
                                    value={newDesc}
                                    onChange={(e) => setNewDesc(e.target.value)}
                                />
                            </div>
                        </div>
                        <DialogFooter>
                            <Button type="button" variant="outline" onClick={() => setIsCreating(false)}>
                                Cancel
                            </Button>
                            <Button type="submit" disabled={isSubmitting}>
                                {isSubmitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                                Create board
                            </Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>
        </div>
    );
}
