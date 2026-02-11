import React, { useState } from 'react';
import useSWR, { mutate } from 'swr';
import { X, Plus, Trash2, UserPlus } from 'lucide-react';

interface MemberManagerProps {
  boardId: string;
  isOpen: boolean;
  onClose: () => void;
}

const fetcher = (url: string) => fetch(url).then((res) => res.json());

export function MemberManager({ boardId, isOpen, onClose }: MemberManagerProps) {
  const { data: boardMembers } = useSWR(`/api/boards/${boardId}/members`, fetcher);
  const { data: allMembers } = useSWR('/api/members', fetcher);
  
  const [selectedMember, setSelectedMember] = useState('');

  if (!isOpen) return null;

  // Filter members who are not yet in the board
  const availableMembers = allMembers?.filter((am: any) => 
    !boardMembers?.some((bm: any) => bm.id === am.id)
  );

  const handleInvite = async () => {
    if (!selectedMember) return;
    await fetch(`/api/boards/${boardId}/members`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ member_id: selectedMember, role: 'editor' }),
    });
    mutate(`/api/boards/${boardId}/members`);
    setSelectedMember('');
  };

  const handleRemove = async (memberId: string) => {
    if (!confirm("Are you sure you want to remove this member?")) return;
    await fetch(`/api/boards/${boardId}/members/${memberId}`, {
      method: 'DELETE',
    });
    mutate(`/api/boards/${boardId}/members`);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl bg-white shadow-2xl dark:bg-zinc-900">
        <div className="flex items-center justify-between border-b p-4 dark:border-zinc-800">
          <h2 className="text-lg font-bold">Manage Members</h2>
          <button onClick={onClose} className="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-zinc-800">
            <X size={20} />
          </button>
        </div>

        <div className="p-6">
          {/* Invite Section */}
          <div className="mb-6 flex gap-2">
            <select
              value={selectedMember}
              onChange={(e) => setSelectedMember(e.target.value)}
              className="flex-1 rounded-lg border bg-gray-50 px-3 py-2 outline-none dark:bg-zinc-800 dark:border-zinc-700"
            >
              <option value="">Select member to invite...</option>
              {availableMembers?.map((m: any) => (
                <option key={m.id} value={m.id}>{m.name} ({m.role})</option>
              ))}
            </select>
            <button
              onClick={handleInvite}
              disabled={!selectedMember}
              className="flex items-center gap-2 rounded-lg bg-rose-500 px-4 py-2 text-white hover:bg-rose-600 disabled:opacity-50"
            >
              <UserPlus size={16} /> Invite
            </button>
          </div>

          {/* Members List */}
          <div className="space-y-3">
            <h3 className="text-sm font-medium text-gray-500">Current Members</h3>
            {boardMembers?.length === 0 && <p className="text-sm text-gray-400">No members yet.</p>}
            {boardMembers?.map((m: any) => (
              <div key={m.id} className="flex items-center justify-between rounded-xl border p-3 dark:border-zinc-800">
                <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-100 text-lg dark:bg-zinc-800">
                    {m.avatar}
                  </div>
                  <div>
                    <div className="font-medium">{m.name}</div>
                    <div className="text-xs text-gray-500 capitalize">{m.role}</div>
                  </div>
                </div>
                <button 
                    onClick={() => handleRemove(m.id)}
                    className="text-gray-400 hover:text-red-500"
                >
                  <Trash2 size={16} />
                </button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
