'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, Copy, Link as LinkIcon, Rocket, Webhook, X, Bot, ShieldCheck, Network } from 'lucide-react';
import { toast } from 'sonner';

type Provider = 'external' | 'clawn';
type Engine = 'webhook' | 'rest' | 'openclaw' | 'picoclaw';
type ClawnTransportMode = 'internal' | 'remote_http' | 'pull';

interface RegisterAgentModalProps {
  boardId: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const defaultCapabilities = {
  can_receive_assignments: true,
  can_post_messages: true,
  can_update_tasks: true,
  can_submit_deliverables: true,
  can_request_review: true,
  can_report_blocked: true,
  can_read_docs: true,
  can_sync_presence: true,
};

export function RegisterAgentModal({ boardId, isOpen, onClose, onSuccess }: RegisterAgentModalProps) {
  const [step, setStep] = useState<1 | 2>(1);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ machine_token: string; agent_id: string } | null>(null);

  const [id, setId] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [roleName, setRoleName] = useState('Worker Agent');
  const [avatar, setAvatar] = useState('🤖');
  const [provider, setProvider] = useState<Provider>('external');
  const [engine, setEngine] = useState<Engine>('webhook');
  const [description, setDescription] = useState('');
  const [endpointUrl, setEndpointUrl] = useState('');
  const [sessionKey, setSessionKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [agentRef, setAgentRef] = useState('');
  const [transportMode, setTransportMode] = useState<ClawnTransportMode>('internal');
  const [boardRole, setBoardRole] = useState('worker');
  const [autoAcceptTasks, setAutoAcceptTasks] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    setStep(1);
    setLoading(false);
    setResult(null);
    setError('');
    setId('');
    setDisplayName('');
    setRoleName('Worker Agent');
    setAvatar('🤖');
    setProvider('external');
    setEngine('webhook');
    setDescription('');
    setEndpointUrl('');
    setSessionKey('');
    setBaseUrl('');
    setAgentRef('');
    setTransportMode('internal');
    setBoardRole('worker');
    setAutoAcceptTasks(true);
  }, [isOpen]);

  useEffect(() => {
    if (provider === 'clawn' && !['openclaw', 'picoclaw'].includes(engine)) {
      setEngine('openclaw');
    }
    if (provider === 'external' && !['webhook', 'rest'].includes(engine)) {
      setEngine('webhook');
    }
  }, [provider, engine]);

  const connectorType = useMemo(() => {
    if (provider === 'clawn') return 'clawn_native';
    if (engine === 'webhook') return 'webhook';
    if (engine === 'rest') return 'rest';
    return 'custom';
  }, [provider, engine]);

  const effectiveTransportMode = useMemo(() => {
    if (provider === 'clawn') return transportMode;
    if (engine === 'rest') return 'pull';
    if (engine === 'webhook') return 'push';
    return 'push';
  }, [provider, engine, transportMode]);

  const endpointHelp = useMemo(() => {
    if (provider === 'clawn' && transportMode === 'internal') {
      return 'Pakai ini kalau runtime OpenClaw/PicoClaw ada di server / private environment yang sama.';
    }
    if (provider === 'clawn' && transportMode === 'remote_http') {
      return 'Pakai ini kalau runtime ada di server lain dan bisa diakses via HTTP(S).';
    }
    if (provider === 'clawn' && transportMode === 'pull') {
      return 'Pull mode cocok buat laptop / device lokal. Runtime nanti polling / relay outbound ke MoziBoard.';
    }
    if (engine === 'rest') {
      return 'REST/Polling bisa didaftarkan dulu. Endpoint ini opsional untuk metadata awal.';
    }
    return 'MoziBoard akan push event assignment ke webhook ini.';
  }, [provider, engine, transportMode]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const registerPayload = {
        id,
        display_name: displayName,
        role_name: roleName,
        avatar,
        provider,
        engine,
        description,
        is_native_clawn: provider === 'clawn',
        connector: {
          connector_type: connectorType,
          transport_mode: effectiveTransportMode,
          auth_type: 'machine_token',
          endpoint_url: endpointUrl,
          base_url: baseUrl,
          agent_ref: agentRef,
          session_key: sessionKey,
          metadata: {
            runtime: provider === 'clawn' ? 'clawn' : 'external',
            board_id: boardId,
          },
        },
        capabilities: defaultCapabilities,
      };

      const regRes = await fetch('/api/agents/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(registerPayload),
      });

      const regData = await regRes.json().catch(() => null);
      if (!regRes.ok) throw new Error(regData?.error || 'Failed to register agent');

      const connRes = await fetch(`/api/agents/${id}/connect-board`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          board_id: boardId,
          board_role: boardRole,
          auto_accept_tasks: autoAcceptTasks,
          permissions: {
            can_comment: true,
            can_update_status: true,
            can_access_docs: true,
            can_create_deliverables: true,
          },
        }),
      });

      const connData = await connRes.json().catch(() => null);
      if (!connRes.ok) throw new Error(connData?.error || 'Failed to connect agent to board');

      setResult({ machine_token: regData.machine_token, agent_id: id });
      setStep(2);
      onSuccess();
      toast.success('Agent berhasil diregister dan dikonek ke board');
    } catch (err: any) {
      const message = err?.message || 'Unknown error';
      setError(message);
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  const copyToken = async () => {
    if (!result?.machine_token) return;
    await navigator.clipboard.writeText(result.machine_token);
    toast.success('Machine token copied');
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/55 p-4 backdrop-blur-sm">
      <div className="w-full max-w-2xl overflow-hidden rounded-2xl border bg-white shadow-2xl dark:border-zinc-800 dark:bg-zinc-900">
        <div className="flex items-center justify-between border-b px-5 py-4 dark:border-zinc-800">
          <div>
            <h2 className="flex items-center gap-2 text-xl font-bold">
              <LinkIcon className="h-5 w-5 text-rose-500" />
              Register Agent
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Tambahin agent baru ke board ini dan langsung siapin connector/runtime auth-nya.
            </p>
          </div>
          <button onClick={onClose} className="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800">
            <X size={20} />
          </button>
        </div>

        {step === 1 ? (
          <form onSubmit={handleSubmit} className="max-h-[85vh] overflow-auto p-5">
            <div className="grid gap-5 lg:grid-cols-[1.2fr_0.8fr]">
              <div className="space-y-5">
                <section className="rounded-2xl border p-4 dark:border-zinc-800">
                  <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                    <Bot className="h-4 w-4 text-rose-500" />
                    Identity
                  </div>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Field label="Agent ID">
                      <input
                        required
                        value={id}
                        onChange={(e) => setId(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                        placeholder="mis. clawn-ops-agent"
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      />
                    </Field>
                    <Field label="Avatar">
                      <input
                        required
                        value={avatar}
                        onChange={(e) => setAvatar(e.target.value)}
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-center text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      />
                    </Field>
                    <Field label="Display Name">
                      <input
                        required
                        value={displayName}
                        onChange={(e) => setDisplayName(e.target.value)}
                        placeholder="mis. Clawn Ops Agent"
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      />
                    </Field>
                    <Field label="Role Name">
                      <input
                        value={roleName}
                        onChange={(e) => setRoleName(e.target.value)}
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      />
                    </Field>
                  </div>
                  <Field label="Description" className="mt-4">
                    <textarea
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      rows={3}
                      placeholder="Ringkasan singkat agent ini ngapain"
                      className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                    />
                  </Field>
                </section>

                <section className="rounded-2xl border p-4 dark:border-zinc-800">
                  <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                    <Rocket className="h-4 w-4 text-rose-500" />
                    Runtime & Connector
                  </div>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Field label="Provider">
                      <select
                        value={provider}
                        onChange={(e) => setProvider(e.target.value as Provider)}
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      >
                        <option value="external">External / Generic</option>
                        <option value="clawn">Clawn Native</option>
                      </select>
                    </Field>
                    <Field label="Engine">
                      <select
                        value={engine}
                        onChange={(e) => setEngine(e.target.value as Engine)}
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      >
                        {provider === 'external' ? (
                          <>
                            <option value="webhook">Webhook Push</option>
                            <option value="rest">REST / Polling</option>
                          </>
                        ) : (
                          <>
                            <option value="openclaw">OpenClaw</option>
                            <option value="picoclaw">PicoClaw</option>
                          </>
                        )}
                      </select>
                    </Field>
                  </div>

                  {provider === 'clawn' && (
                    <>
                      <Field label="Clawn Native Mode" className="mt-4">
                        <div className="grid gap-2">
                          <TransportOption
                            active={transportMode === 'internal'}
                            title="This server / internal"
                            desc="Runtime ada di host/private environment yang sama."
                            onClick={() => setTransportMode('internal')}
                          />
                          <TransportOption
                            active={transportMode === 'remote_http'}
                            title="Remote OpenClaw URL"
                            desc="Runtime ada di server lain dan bisa diakses via HTTP(S)."
                            onClick={() => setTransportMode('remote_http')}
                          />
                          <TransportOption
                            active={transportMode === 'pull'}
                            title="Local / Pull mode"
                            desc="Untuk laptop/device lokal. Runtime polling atau relay outbound nanti."
                            onClick={() => setTransportMode('pull')}
                          />
                        </div>
                      </Field>

                      {(transportMode === 'internal' || transportMode === 'remote_http' || transportMode === 'pull') && (
                        <Field label="Agent Ref" className="mt-4">
                          <input
                            value={agentRef}
                            onChange={(e) => setAgentRef(e.target.value)}
                            placeholder="mis. mozi-prod / remote-worker-01 / local-macbook"
                            className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                          />
                        </Field>
                      )}

                      {transportMode === 'remote_http' && (
                        <Field label="Base URL" className="mt-4">
                          <input
                            type="url"
                            value={baseUrl}
                            onChange={(e) => setBaseUrl(e.target.value)}
                            required={transportMode === 'remote_http'}
                            placeholder="https://runtime.example.com"
                            className="w-full rounded-xl border bg-transparent px-3 py-2 font-mono text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                          />
                        </Field>
                      )}

                      {(transportMode === 'internal' || transportMode === 'remote_http') && (
                        <Field label="Session Key" className="mt-4">
                          <input
                            value={sessionKey}
                            onChange={(e) => setSessionKey(e.target.value)}
                            placeholder="mis. agent:main:main"
                            className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                          />
                        </Field>
                      )}
                    </>
                  )}

                  {provider === 'external' && (
                    <Field label="Endpoint URL" className="mt-4">
                      <div className="relative">
                        <Webhook className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
                        <input
                          type="url"
                          value={endpointUrl}
                          onChange={(e) => setEndpointUrl(e.target.value)}
                          required={engine === 'webhook'}
                          placeholder="https://agent.example.com/hook"
                          className="w-full rounded-xl border bg-transparent py-2 pl-9 pr-3 font-mono text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                        />
                      </div>
                    </Field>
                  )}

                  <p className="mt-3 text-xs text-muted-foreground">{endpointHelp}</p>
                </section>
              </div>

              <div className="space-y-5">
                <section className="rounded-2xl border p-4 dark:border-zinc-800">
                  <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                    <ShieldCheck className="h-4 w-4 text-rose-500" />
                    Board Connection
                  </div>
                  <div className="space-y-4">
                    <Field label="Board Role">
                      <select
                        value={boardRole}
                        onChange={(e) => setBoardRole(e.target.value)}
                        className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      >
                        <option value="worker">Worker</option>
                        <option value="reviewer">Reviewer</option>
                        <option value="lead">Lead</option>
                      </select>
                    </Field>
                    <label className="flex items-start gap-3 rounded-xl border p-3 text-sm dark:border-zinc-800">
                      <input
                        type="checkbox"
                        checked={autoAcceptTasks}
                        onChange={(e) => setAutoAcceptTasks(e.target.checked)}
                        className="mt-0.5"
                      />
                      <div>
                        <div className="font-medium">Auto-accept tasks</div>
                        <div className="text-xs text-muted-foreground">Kalau aktif, agent dianggap siap nerima assignment secara otomatis.</div>
                      </div>
                    </label>
                  </div>
                </section>

                <section className="rounded-2xl border bg-zinc-50 p-4 text-sm dark:border-zinc-800 dark:bg-zinc-950/50">
                  <div className="flex items-center gap-2 font-semibold"><Network className="h-4 w-4 text-rose-500" /> Preview</div>
                  <div className="mt-3 space-y-2 text-muted-foreground">
                    <div><span className="font-medium text-foreground">Connector:</span> {connectorType}</div>
                    <div><span className="font-medium text-foreground">Transport:</span> {effectiveTransportMode}</div>
                    <div><span className="font-medium text-foreground">Runtime:</span> {provider} / {engine}</div>
                    {!!baseUrl && <div><span className="font-medium text-foreground">Base URL:</span> {baseUrl}</div>}
                    {!!agentRef && <div><span className="font-medium text-foreground">Agent Ref:</span> {agentRef}</div>}
                    <div><span className="font-medium text-foreground">Board:</span> {boardId}</div>
                    <div><span className="font-medium text-foreground">Auth:</span> machine token</div>
                  </div>
                </section>

                {error && (
                  <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300">
                    {error}
                  </div>
                )}
              </div>
            </div>

            <div className="mt-5 flex justify-end gap-2 border-t pt-4 dark:border-zinc-800">
              <button type="button" onClick={onClose} className="rounded-xl px-4 py-2 text-sm font-medium hover:bg-zinc-100 dark:hover:bg-zinc-800">
                Cancel
              </button>
              <button
                type="submit"
                disabled={loading || !id || !displayName}
                className="rounded-xl bg-rose-600 px-4 py-2 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-50"
              >
                {loading ? 'Registering...' : 'Register & Connect'}
              </button>
            </div>
          </form>
        ) : (
          <div className="p-6">
            <div className="rounded-2xl border p-6 text-center dark:border-zinc-800">
              <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400">
                <CheckCircle2 size={32} />
              </div>
              <h3 className="mt-4 text-xl font-bold">Agent connected ke board</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                Agent <strong>{displayName}</strong> udah diregister dan siap dipakai di board ini.
              </p>
            </div>

            <div className="mt-5 rounded-2xl border bg-zinc-50 p-4 dark:border-zinc-800 dark:bg-zinc-950/40">
              <div className="text-xs font-bold uppercase tracking-wider text-muted-foreground">Machine Token</div>
              <div className="mt-3 flex items-center gap-2">
                <code className="block flex-1 overflow-x-auto rounded-xl border bg-white p-3 text-xs dark:border-zinc-800 dark:bg-zinc-900">
                  {result?.machine_token}
                </code>
                <button onClick={copyToken} className="rounded-xl border bg-white p-3 hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:bg-zinc-800" title="Copy token">
                  <Copy size={16} />
                </button>
              </div>
              <p className="mt-2 text-xs text-amber-600 dark:text-amber-400">
                Simpan token ini sekarang. Setelah modal ditutup, token plaintext gak bakal ditampilin lagi.
              </p>
            </div>

            <div className="mt-5 flex justify-end gap-2">
              <button
                onClick={() => {
                  setStep(1);
                  setResult(null);
                  setError('');
                }}
                className="rounded-xl px-4 py-2 text-sm font-medium hover:bg-zinc-100 dark:hover:bg-zinc-800"
              >
                Register another
              </button>
              <button onClick={onClose} className="rounded-xl bg-black px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 dark:bg-white dark:text-black dark:hover:bg-zinc-200">
                Done
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Field({ label, children, className = '' }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={className}>
      <label className="mb-1.5 block text-xs font-medium text-muted-foreground">{label}</label>
      {children}
    </div>
  );
}

function TransportOption({
  active,
  title,
  desc,
  onClick,
}: {
  active: boolean;
  title: string;
  desc: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-xl border p-3 text-left transition ${active ? 'border-rose-500 bg-rose-50 dark:border-rose-400 dark:bg-rose-950/20' : 'hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-900'}`}
    >
      <div className="font-medium">{title}</div>
      <div className="mt-1 text-xs text-muted-foreground">{desc}</div>
    </button>
  );
}
