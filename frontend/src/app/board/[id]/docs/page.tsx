'use client';

import { useParams, useRouter } from 'next/navigation';
import { KnowledgeBase } from '@/components/KnowledgeBase';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import { BoardTopbar } from '@/components/BoardTopbar';

export default function DocsPage() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);

  return (
    <div className="flex h-full w-full flex-col">
      <BoardTopbar
        boardId={id}
        section="docs"
        onMembersClick={() => setShowMembers(true)}
        onTaskSelect={(task) => router.push(`/board/${id}`)}
        onDocSelect={(docId) => router.push(`/board/${id}/docs?doc=${docId}`)}
      />

      <div className="relative flex-1 overflow-hidden">
        <KnowledgeBase boardId={id} />
      </div>

      <MemberManager
        boardId={id}
        isOpen={showMembers}
        onClose={() => setShowMembers(false)}
      />
    </div>
  );
}
