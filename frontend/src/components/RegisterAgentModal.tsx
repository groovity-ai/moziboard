import React, { useState } from 'react';
import { X, Server, Webhook, Link as LinkIcon, CheckCircle2, Copy } from 'lucide-react';
import { toast } from 'sonner';

interface RegisterAgentModalProps {
  boardId: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export function RegisterAgentModal({ boardId, isOpen, onClose, onSuccess }: RegisterAgentModalProps) {
  const [step, setStep] = useState<1 | 2>(1);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ machine_token: string, agent_id: string } | null>(null);

  // Form State
  const [id, setId] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [roleName, setRoleName] = useState('Worker Agent');
  const [avatar, setAvatar] = useState('🤖');
  const [provider, setProvider] = useState<'external' | 'clawn'>('external');
  const [engine, setEngine] = useState<'webhook' | 'openclaw' | 'picoclaw' | 'rest'>('webhook');
  const [endpointUrl, setEndpointUrl] = useState('');

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      // 1. Register Agent
      const regRes = await fetch('/api/agents/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id,
          display_name: displayName,
          role_name: roleName,
          avatar,
          provider,
          engine,
          is_native_clawn: provider === 'clawn',
          connector: {
            connector_type: engine === 'webhook' ? 'webhook' : 'custom',
            auth_type: 'machine_token',
            endpoint_url: endpointUrl
          },
          capabilities: {
            can_receive_assignments: true,
            can_post_messages: true,
            can_update_tasks: true,
            can_submit_deliverables: true,
            can_request_review: true,
            can_report_blocked: true,
            can_read_docs: true,
            can_sync_presence: true
          }
        }),
      });

      if (!regRes.ok) throw new Error(await regRes.text());
      const regData = await regRes.json();

      // 2. Connect to Board
      const connRes = await fetch(`/api/agents/${id}/connect-board`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          board_id: boardId,
          board_role: 'worker',
          auto_accept_tasks: true
        }),
      });

      if (!connRes.ok) throw new Error(await connRes.text());

      setResult({
        machine_token: regData.machine_token,
        agent_id: id
      });
      setStep(2);
      onSuccess();
      toast.success('Agent registered & connected successfully!');

    } catch (err: any) {
      toast.error('Failed: ' + err.message);
    } finally {
      setLoading(false);
    }
  };

  const copyToken = () => {
    if (result) {
      navigator.clipboard.writeText(result.machine_token);
      toast.success('Token copied to clipboard');
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-lg rounded-2xl bg-white shadow-2xl dark:bg-zinc-900 border dark:border-zinc-800">
        <div className="flex items-center justify-between border-b p-4 dark:border-zinc-800">
          <h2 className="text-xl font-bold flex items-center gap-2">
            <LinkIcon className="w-5 h-5 text-rose-500" />
            Connect New Agent
          </h2>
          <button onClick={onClose} className="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-zinc-800">
            <X size={20} />
          </button>
        </div>

        {step === 1 ? (
          <form onSubmit={handleSubmit} className="p-6 space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">Provider</label>
                <select
                  value={provider}
                  onChange={(e: any) => setProvider(e.target.value)}
                  className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                >
                  <option value="external">External / Custom</option>
                  <option value="clawn">Clawn Native</option>
                </select>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">Engine Type</label>
                <select
                  value={engine}
                  onChange={(e: any) => setEngine(e.target.value)}
                  className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                >
                  <option value="webhook">Webhook (Push)</option>
                  <option value="rest">REST (Polling)</option>
                  {provider === 'clawn' && (
                    <>
                      <option value="openclaw">OpenClaw</option>
                      <option value="picoclaw">PicoClaw</option>
                    </>
                  )}
                </select>
              </div>
            </div>

            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Unique Agent ID (slug)</label>
              <input
                required
                type="text"
                placeholder="e.g., custom-worker-1"
                value={id}
                onChange={(e) => setId(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
              />
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div className="col-span-2 space-y-1">
                <label className="text-xs font-medium text-muted-foreground">Display Name</label>
                <input
                  required
                  type="text"
                  placeholder="e.g., Auto Deployer"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">Avatar Emoji</label>
                <input
                  required
                  type="text"
                  value={avatar}
                  onChange={(e) => setAvatar(e.target.value)}
                  className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700 text-center"
                />
              </div>
            </div>

            {engine === 'webhook' && (
              <div className="space-y-1 pt-2">
                <label className="text-xs font-medium text-muted-foreground flex items-center gap-1">
                  <Webhook size={14} /> Webhook Callback URL
                </label>
                <input
                  required={engine === 'webhook'}
                  type="url"
                  placeholder="https://your-server.com/api/agent/webhook"
                  value={endpointUrl}
                  onChange={(e) => setEndpointUrl(e.target.value)}
                  className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700 font-mono text-xs"
                />
                <p className="text-[10px] text-muted-foreground">MoziBoard will push assignment events to this URL.</p>
              </div>
            )}

            <div className="pt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-sm font-medium rounded-md hover:bg-zinc-100 dark:hover:bg-zinc-800"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={loading}
                className="px-4 py-2 text-sm font-medium rounded-md bg-rose-600 text-white hover:bg-rose-700 disabled:opacity-50 flex items-center gap-2"
              >
                {loading ? 'Connecting...' : 'Register & Connect'}
              </button>
            </div>
          </form>
        ) : (
          <div className="p-8 text-center space-y-6">
            <div className="mx-auto w-16 h-16 bg-green-100 text-green-600 rounded-full flex items-center justify-center dark:bg-green-900/30 dark:text-green-500">
              <CheckCircle2 size={32} />
            </div>
            <div>
              <h3 className="text-xl font-bold">Agent Connected!</h3>
              <p className="text-sm text-muted-foreground mt-2">
                The agent <strong>{displayName}</strong> is now registered and bound to this board.
              </p>
            </div>

            <div className="bg-zinc-50 dark:bg-zinc-950 border dark:border-zinc-800 rounded-xl p-4 text-left">
              <label className="text-xs font-bold text-muted-foreground uppercase tracking-wider">Machine Token (Bearer)</label>
              <div className="mt-2 flex items-center gap-2">
                <code className="flex-1 block p-2 bg-white dark:bg-zinc-900 border rounded text-xs overflow-hidden text-ellipsis">
                  {result?.machine_token}
                </code>
                <button
                  onClick={copyToken}
                  className="p-2 bg-white dark:bg-zinc-800 border rounded hover:bg-zinc-50 dark:hover:bg-zinc-700 text-zinc-500"
                  title="Copy Token"
                >
                  <Copy size={16} />
                </button>
              </div>
              <p className="text-[11px] text-rose-500 mt-2">
                ⚠️ Store this token securely. It will not be shown again. Use it as a Bearer token for all `/api/runtime/*` requests.
              </p>
            </div>

            <button
              onClick={onClose}
              className="w-full px-4 py-2 text-sm font-medium rounded-md bg-black text-white hover:bg-zinc-800 dark:bg-white dark:text-black dark:hover:bg-zinc-200"
            >
              Done
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
