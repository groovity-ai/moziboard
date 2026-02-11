import React from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { TaskCard } from './TaskCard';
import { ListType, Task } from './Board';
import { mutate } from 'swr';

interface ListContainerProps {
  list: ListType;
  boardId: string;
  onTaskClick?: (task: Task) => void;
}

export function ListContainer({ list, boardId, onTaskClick }: ListContainerProps) {
  const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
    id: list.id,
    data: {
      type: 'list',
      list,
    },
  });

  const style = {
    transition,
    transform: CSS.Transform.toString(transform),
  };

  const handleAddTask = async () => {
    const title = prompt("Enter task title:");
    if (!title) return;

    await fetch('/api/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title,
        description: 'New task created',
        list_id: list.id,
        board_id: boardId, // String UUID
        position: list.tasks.length + 1,
      }),
    });
    mutate(`/api/boards/${boardId}/tasks`);
  };

  if (isDragging) {
    return (
      <div
        ref={setNodeRef}
        style={style}
        className="w-[350px] shrink-0 rounded-md border-2 border-dashed border-gray-400 bg-gray-100 opacity-50"
      />
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="flex h-fit w-[350px] shrink-0 flex-col gap-4 rounded-xl bg-gray-100 p-4 shadow-sm dark:bg-zinc-900"
    >
      <div
        {...attributes}
        {...listeners}
        className="flex cursor-grab items-center justify-between text-lg font-bold"
      >
        <span>{list.title}</span>
        <span className="rounded-full bg-gray-200 px-2 py-1 text-sm dark:bg-zinc-800">
          {list.tasks.length}
        </span>
      </div>

      <div className="flex flex-col gap-2">
        <SortableContext items={list.tasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
          {list.tasks.map((task) => (
            <TaskCard key={task.id} task={task} onClick={() => onTaskClick?.(task)} />
          ))}
        </SortableContext>
      </div>
      
      <button 
        onClick={handleAddTask}
        className="flex w-full items-center gap-2 rounded-md p-2 hover:bg-gray-200 dark:hover:bg-zinc-800"
      >
        <span className="text-xl">+</span> Add Task
      </button>
    </div>
  );
}
