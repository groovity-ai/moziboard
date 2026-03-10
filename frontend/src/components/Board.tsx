import React, { useState, useEffect } from 'react';
import useSWR, { mutate } from 'swr';
import {
  DndContext,
  DragOverlay,
  closestCorners,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragStartEvent,
  DragEndEvent,
  DragOverEvent,
} from '@dnd-kit/core';
import {
  sortableKeyboardCoordinates,
} from '@dnd-kit/sortable';
import { ListContainer } from './ListContainer';
import { TaskCard } from './TaskCard';
import { TaskDetailModal } from './TaskDetailModal';
import { SearchBar } from './SearchBar';

export type Task = {
  id: string | number;
  board_id: string;
  title: string;
  description?: string;
  list_id: string;
  position: number;
  assignee_id?: string;
  status?: string;
  blocked_reason?: string;
};

export type ListType = {
  id: string;
  title: string;
  tasks: Task[];
};

const fetcher = (url: string) => fetch(url).then((res) => res.json());

const defaultLists = [
  { id: 'backlog', title: 'Backlog', tasks: [] },
  { id: 'todo', title: 'To Do', tasks: [] },
  { id: 'doing', title: 'In Progress', tasks: [] },
  { id: 'qa', title: 'QA / Review', tasks: [] },
  { id: 'done', title: 'Done', tasks: [] },
];

interface BoardProps {
  boardId: string;
  externalTask?: Task | null;
  onTaskHandled?: () => void;
}

function buildListsFromTasks(tasks: Task[] | undefined): ListType[] {
  return defaultLists.map((list) => ({
    ...list,
    tasks: (tasks || [])
      .filter((t) => t.list_id === list.id)
      .sort((a, b) => a.position - b.position),
  }));
}

function reindexListTasks(tasks: Task[], listId: string): Task[] {
  return tasks.map((task, index) => ({
    ...task,
    list_id: listId,
    position: index + 1,
  }));
}

function findListIdForDrop(lists: ListType[], overId: string | number | null | undefined): string | null {
  if (!overId) return null;
  const overStr = String(overId);
  if (lists.some((list) => list.id === overStr)) return overStr;
  for (const list of lists) {
    if (list.tasks.some((task) => String(task.id) === overStr)) return list.id;
  }
  return null;
}

function moveTaskInLists(lists: ListType[], activeId: string, overId: string): { nextLists: ListType[]; changedTasks: Task[] } {
  const cloned = lists.map((list) => ({ ...list, tasks: [...list.tasks] }));

  let sourceListIndex = -1;
  let sourceTaskIndex = -1;
  for (let i = 0; i < cloned.length; i++) {
    const idx = cloned[i].tasks.findIndex((task) => String(task.id) === activeId);
    if (idx >= 0) {
      sourceListIndex = i;
      sourceTaskIndex = idx;
      break;
    }
  }

  if (sourceListIndex === -1 || sourceTaskIndex === -1) {
    return { nextLists: lists, changedTasks: [] };
  }

  const sourceList = cloned[sourceListIndex];
  const [movedTask] = sourceList.tasks.splice(sourceTaskIndex, 1);
  const targetListId = findListIdForDrop(cloned, overId) || movedTask.list_id;
  const targetListIndex = cloned.findIndex((list) => list.id === targetListId);
  if (targetListIndex === -1) {
    return { nextLists: lists, changedTasks: [] };
  }

  const targetList = cloned[targetListIndex];
  let insertIndex = targetList.tasks.length;
  if (overId === targetList.id) {
    insertIndex = targetList.tasks.length;
  } else {
    const overTaskIndex = targetList.tasks.findIndex((task) => String(task.id) === overId);
    if (overTaskIndex >= 0) {
      insertIndex = overTaskIndex;
    }
  }

  targetList.tasks.splice(insertIndex, 0, { ...movedTask, list_id: targetList.id });

  const reindexedSource = reindexListTasks(sourceList.tasks, sourceList.id);
  const reindexedTarget = sourceList.id === targetList.id
    ? reindexListTasks(targetList.tasks, targetList.id)
    : reindexListTasks(targetList.tasks, targetList.id);

  cloned[sourceListIndex] = {
    ...sourceList,
    tasks: sourceList.id === targetList.id ? reindexedTarget : reindexedSource,
  };
  cloned[targetListIndex] = {
    ...targetList,
    tasks: reindexedTarget,
  };

  const changedMap = new Map<string, Task>();
  cloned[sourceListIndex].tasks.forEach((task) => changedMap.set(String(task.id), task));
  cloned[targetListIndex].tasks.forEach((task) => changedMap.set(String(task.id), task));

  return {
    nextLists: cloned,
    changedTasks: Array.from(changedMap.values()),
  };
}

export function Board({ boardId, externalTask, onTaskHandled }: BoardProps) {
  const { data: tasks } = useSWR<Task[]>(`/api/boards/${boardId}/tasks`, fetcher, {
    revalidateOnFocus: false,
    refreshInterval: 0,
  });

  const [lists, setLists] = useState<ListType[]>(defaultLists);
  const [activeTask, setActiveTask] = useState<Task | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [activeListId, setActiveListId] = useState<string | null>(null);
  const [lastSavedAt, setLastSavedAt] = useState<number | null>(null);
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');

  useEffect(() => {
    if (externalTask) {
      setSelectedTask(externalTask);
      onTaskHandled?.();
    }
  }, [externalTask, onTaskHandled]);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let reconnectTimeout: ReturnType<typeof setTimeout>;
    let reconnectDelay = 1000;
    const MAX_RECONNECT_DELAY = 30000;

    function connect() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws`;
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        reconnectDelay = 1000;
      };
      ws.onmessage = (event) => {
        if (event.data === 'UPDATE') {
          mutate(`/api/boards/${boardId}/tasks`);
        }
      };
      ws.onclose = () => {
        reconnectTimeout = setTimeout(() => {
          reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY);
          connect();
        }, reconnectDelay);
      };
      ws.onerror = () => ws?.close();
    }

    connect();

    return () => {
      clearTimeout(reconnectTimeout);
      ws?.close();
    };
  }, [boardId]);

  useEffect(() => {
    if (tasks) {
      setLists(buildListsFromTasks(tasks));
    }
  }, [tasks]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  async function persistTask(task: Task) {
    await fetch(`/api/tasks/${task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...task, board_id: boardId }),
    });
  }

  const handleDragStart = (event: DragStartEvent) => {
    const task = tasks?.find((t) => String(t.id) === String(event.active.id));
    if (task) {
      setActiveTask(task);
      setActiveListId(task.list_id);
      setSaveState('idle');
    }
  };

  const handleDragOver = (event: DragOverEvent) => {
    const { active, over } = event;
    if (!over) return;

    const activeId = String(active.id);
    const overId = String(over.id);
    if (activeId === overId) return;

    const { nextLists } = moveTaskInLists(lists, activeId, overId);
    if (nextLists !== lists) {
      setLists(nextLists);
      setActiveListId(findListIdForDrop(nextLists, overId));
    }
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    setActiveTask(null);
    setActiveListId(null);

    if (!over) return;
    const activeId = String(active.id);
    const overId = String(over.id);
    if (activeId === overId) return;

    const { nextLists, changedTasks } = moveTaskInLists(lists, activeId, overId);
    if (changedTasks.length === 0) return;

    const optimisticTasks = nextLists.flatMap((list) => list.tasks);
    const snapshotLists = lists;
    setLists(nextLists);
    setSaveState('saving');

    mutate(`/api/boards/${boardId}/tasks`, optimisticTasks, false);

    try {
      await Promise.all(changedTasks.map((task) => persistTask(task)));
      setLastSavedAt(Date.now());
      setSaveState('saved');
      mutate(`/api/boards/${boardId}/tasks`);
    } catch (error) {
      setSaveState('error');
      setLists(snapshotLists);
      mutate(`/api/boards/${boardId}/tasks`);
    }
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      <div className="flex h-full w-full flex-col p-4">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div className="text-[10px] font-medium uppercase tracking-widest text-muted-foreground/60">
            {saveState === 'saving' && <span className="flex items-center gap-1.5"><span className="h-1.5 w-1.5 animate-pulse rounded-full bg-rose-500" /> Syncing…</span>}
            {saveState === 'saved' && <span>Board Synced · {lastSavedAt ? new Date(lastSavedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''}</span>}
            {saveState === 'error' && <span className="text-rose-500">Sync Error</span>}
            {saveState === 'idle' && <span>Board Operational</span>}
          </div>
        </div>

        <div className="flex h-full w-full gap-4 overflow-x-auto pb-2">
          {lists.map((list) => (
            <ListContainer
              key={list.id}
              list={list}
              boardId={boardId}
              isActiveDropTarget={activeListId === list.id}
              onTaskClick={(task) => setSelectedTask(task)}
            />
          ))}
          <DragOverlay>
            {activeTask ? <TaskCard task={activeTask} dragOnly /> : null}
          </DragOverlay>
        </div>
      </div>

      {selectedTask && (
        <TaskDetailModal
          task={selectedTask}
          isOpen={!!selectedTask}
          onClose={() => setSelectedTask(null)}
        />
      )}
    </DndContext>
  );
}
