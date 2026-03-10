'use client';

import useSWR from 'swr';
import Link from 'next/link';
import { ArrowLeft, Search, Layout, FileText, Users } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { SearchBar } from '@/components/SearchBar';
import { Button } from '@/components/ui/button';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

type BoardItem = {
  id: string;
  title: string;
  description?: string;
};

interface BoardTopbarProps {
  boardId: string;
  section: 'board' | 'docs';
  onMembersClick: () => void;
  onTaskSelect?: (task: any) => void;
  onDocSelect?: (docId: string) => void;
}

export function BoardTopbar({ boardId, section, onMembersClick, onTaskSelect, onDocSelect }: BoardTopbarProps) {
  const { data: boards } = useSWR<BoardItem[]>('/api/boards', fetcher, {
    revalidateOnFocus: false,
  });

  const board = boards?.find((item) => item.id === boardId);
  const boardTitle = board?.title || 'Workspace';
  const sectionLabel = section === 'board' ? 'Kanban Board' : 'Knowledge Base';
  const SectionIcon = section === 'board' ? Layout : FileText;
  const sectionTone = section === 'board' ? 'text-rose-500' : 'text-emerald-500';

  return (
    <header className="relative z-50 flex h-16 shrink-0 items-center justify-between gap-4 border-b bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <SidebarTrigger />

        <Link
          href="/"
          className="inline-flex h-9 items-center gap-2 rounded-xl border bg-background px-3 text-sm font-medium text-muted-foreground transition hover:border-foreground/20 hover:bg-muted hover:text-foreground dark:border-zinc-800 dark:bg-zinc-950"
        >
          <ArrowLeft size={16} />
          <span className="hidden sm:inline">Boards</span>
        </Link>

        <div className="hidden h-6 w-px bg-border md:block" />

        <div className="min-w-0 pr-2">
          <div className="truncate text-sm font-semibold tracking-tight">{boardTitle}</div>
          <div className="mt-0.5 flex items-center gap-1 text-[11px] text-muted-foreground">
            <SectionIcon className={`h-3.5 w-3.5 ${sectionTone}`} />
            <span>{sectionLabel}</span>
          </div>
        </div>

        <div className="hidden max-w-xl flex-1 px-2 md:flex">
          <SearchBar
            boardId={boardId}
            onTaskSelect={onTaskSelect}
            onDocSelect={onDocSelect}
          />
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={onMembersClick}
          className="hidden items-center gap-2 sm:flex"
        >
          <Users size={16} /> <span className="text-xs">Members</span>
        </Button>
        <div className="md:hidden">
          <Button variant="ghost" size="icon" className="h-9 w-9">
            <Search size={18} />
          </Button>
        </div>
      </div>
    </header>
  );
}
