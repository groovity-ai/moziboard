'use client';

import { useParams, useRouter } from 'next/navigation';
import { Board } from '@/components/Board';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import { BoardTopbar } from '@/components/BoardTopbar';

export default function BoardPage() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);
  const [selectedTask, setSelectedTask] = useState<any>(null);

  return (
    <div className="flex h-full w-full flex-col">
      <BoardTopbar
        boardId={id}
        section="board"
        onMembersClick={() => setShowMembers(true)}
        onTaskSelect={(task) => setSelectedTask(task)}
        onDocSelect={(docId) => router.push(`/board/${id}/docs?doc=${docId}`)}
      />

      <div className="relative flex-1 overflow-hidden">
        <div className="h-full w-full overflow-hidden">
          <Board boardId={id} externalTask={selectedTask} onTaskHandled={() => setSelectedTask(null)} />
        </div>
      </div>

      <MemberManager
        boardId={id}
        isOpen={showMembers}
        onClose={() => setShowMembers(false)}
      />
    </div>
  );
}
