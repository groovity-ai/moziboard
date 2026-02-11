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
  DragOverEvent,
  DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  horizontalListSortingStrategy,
} from '@dnd-kit/sortable';
import { ListContainer } from './ListContainer';
import { TaskCard } from './TaskCard';
import { TaskDetailModal } from './TaskDetailModal';
import { SearchBar } from './SearchBar';

export type Task = {
  id: string | number;
  board_id: string; // UUID
  title: string;
  description?: string;
  list_id: string;
  position: number;
  assignee_id?: string;
};

export type ListType = {
  id: string;
  title: string;
  tasks: Task[];
};

const fetcher = (url: string) => fetch(url).then((res) => res.json());

const defaultLists = [
  { id: 'todo', title: 'To Do', tasks: [] },
  { id: 'doing', title: 'In Progress', tasks: [] },
  { id: 'done', title: 'Done', tasks: [] },
];

interface BoardProps {
  boardId: string;
}

export function Board({ boardId }: BoardProps) {
  const { data: tasks, error } = useSWR<Task[]>(`/api/boards/${boardId}/tasks`, fetcher, {
    revalidateOnFocus: false,
    refreshInterval: 0
  });

  const [lists, setLists] = useState<ListType[]>(defaultLists);
  const [activeTask, setActiveTask] = useState<Task | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);

  // WebSocket Setup with reconnection
  useEffect(() => {
    let ws: WebSocket | null = null;
    let reconnectTimeout: ReturnType<typeof setTimeout>;
    let reconnectDelay = 1000;
    const MAX_RECONNECT_DELAY = 30000;

    function connect() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws`;

      console.log("Connecting to WS:", wsUrl);
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        console.log("✅ WS Connected");
        reconnectDelay = 1000; // Reset on successful connect
      };
      ws.onmessage = (event) => {
        console.log("📩 WS Update:", event.data);
        if (event.data === "UPDATE") {
          mutate(`/api/boards/${boardId}/tasks`);
        }
      };
      ws.onclose = () => {
        console.log(`❌ WS Disconnected. Reconnecting in ${reconnectDelay / 1000}s...`);
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
      setLists((prevLists) => {
        return prevLists.map((list) => ({
          ...list,
          tasks: tasks.filter((t) => t.list_id === list.id).sort((a, b) => a.position - b.position),
        }));
      });
    }
  }, [tasks]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  async function updateTask(task: Task) {
    await fetch(`/api/tasks/${task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...task, board_id: boardId }),
    });
  }

  const handleDragStart = (event: DragStartEvent) => {
    const { active } = event;
    const task = tasks?.find((t) => String(t.id) === String(active.id));
    if (task) setActiveTask(task);
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over) return;
    const activeId = active.id;
    const overId = over.id;
    let task = tasks?.find((t) => String(t.id) === String(activeId));
    if (!task) return;

    let newListId = task.list_id;
    if (defaultLists.some(l => l.id === overId)) {
      newListId = overId as string;
    } else {
      const overTask = tasks?.find(t => String(t.id) === String(overId));
      if (overTask) newListId = overTask.list_id;
    }

    if (task.list_id !== newListId) {
      await updateTask({ ...task, list_id: newListId });
    }
    setActiveTask(null);
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <div className="flex h-full w-full flex-col p-4">
        <div className="mb-4 flex w-full justify-center">
          <SearchBar onTaskSelect={(task) => setSelectedTask(task)} />
        </div>

        <div className="flex h-full w-full gap-4 overflow-x-auto">
          {lists.map((list) => (
            <ListContainer
              key={list.id}
              list={list}
              boardId={boardId}
              onTaskClick={(task) => setSelectedTask(task)}
            />
          ))}
          <DragOverlay>{activeTask ? <TaskCard task={activeTask} /> : null}</DragOverlay>
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
