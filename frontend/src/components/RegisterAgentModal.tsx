'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, Copy, Link as LinkIcon, Rocket, Webhook, X, Bot, ShieldCheck, Network, Search, FolderGit2, PlugZap } from 'lucide-react';
import { toast } from 'sonner';
import { buildAuthHeaders } from '@/lib/auth';

type Provider = 'external' | 'clawn';
type Engine = 'webhook' | 'rest' | 'openclaw' | 'picoclaw';
type ClawnTransportMode = 'internal' | 'remote_http' | 'pull';

interface RegisterAgentModalProps {
  boardId: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

type ClawnProject = {
  project_id: string;
  display_name: string;
  owner_user_id?: string;
  engine?: string;
  plan?: string;
  status?: string;
  container_id?: string;
  container_name?: string;
  capabilities?: string[];
  is_connectable: boolean;
  connect_reason?: string;
  already_connected?: boolean;
  existing_agent_id?: string;
};

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
  const [clawnProjects, setClawnProjects] = useState<ClawnProject[]>([]);
  const [clawnLoading, setClawnLoading] = useState(false);
  const [clawnSearch, setClawnSearch] = useState('');
  const [selectedClawnProjectId, setSelectedClawnProjectId] = useState('');

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
    setClawnProjects([]);
    setClawnLoading(false);
    setClawnSearch('');
    setSelectedClawnProjectId('');
  }, [isOpen]);

  useEffect(() => {
    if (provider === 'clawn' && !['openclaw', 'picoclaw'].includes(engine)) {
      setEngine('openclaw');
    }
    if (provider === 'external' && !['webhook', 'rest'].includes(engine)) {
      setEngine('webhook');
    }
  }, [provider, engine]);

  useEffect(() => {
    if (!isOpen || provider !== 'clawn') return;
    let cancelled = false;
    const run = async () => {
      try {
        setClawnLoading(true);
        const res = await fetch(`/api/integrations/clawn/projects?board_id=${boardId}`, {
          headers: buildAuthHeaders(),
          credentials: 'include',
        });
        const data = await res.json().catch(() => []);
        if (!res.ok) throw new Error(data?.error || 'Failed to load Clawn projects');
        if (!cancelled) setClawnProjects(Array.isArray(data) ? data : []);
      } catch (err: any) {
        if (!cancelled) {
          setError(err?.message || 'Failed to load Clawn projects');
          toast.error(err?.message || 'Failed to load Clawn projects');
        }
      } finally {
        if (!cancelled) setClawnLoading(false);
      }
    };
    run();
    return () => {
      cancelled = true;
    };
  }, [isOpen, provider, boardId]);

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
    if (provider === 'clawn') {
      return 'Untuk Clawn picker, connector diisi otomatis oleh sistem berdasarkan project yang lu pilih.';
    }
    if (engine === 'rest') {
      return 'REST/Polling bisa didaftarkan dulu. Endpoint ini opsional untuk metadata awal.';
    }
    return 'ClawnBoard akan push event assignment ke webhook ini.';
  }, [provider, engine]);

  const filteredClawnProjects = useMemo(() => {
    const q = clawnSearch.trim().toLowerCase();
    return clawnProjects.filter((item) => {
      if (!q) return true;
      return [item.display_name, item.project_id, item.engine, item.plan, item.status, ...(item.capabilities || [])]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(q);
    });
  }, [clawnProjects, clawnSearch]);

  const selectedClawnProject = useMemo(
    () => clawnProjects.find((item) => item.project_id === selectedClawnProjectId) || null,
    [clawnProjects, selectedClawnProjectId]
  );

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      if (provider === 'clawn') {
        if (!selectedClawnProjectId) throw new Error('Pilih project Clawn dulu');
        const connRes = await fetch('/api/integrations/clawn/connect', {
          method: 'POST',
          headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
          credentials: 'include',
          body: JSON.stringify({
            board_id: boardId,
            clawn_project_id: selectedClawnProjectId,
            board_role: boardRole,
            auto_accept_tasks: autoAcceptTasks,
            can_comment: true,
            can_update_status: true,
            can_access_docs: true,
            can_create_deliverables: true,
          }),
        });
        const connData = await connRes.json().catch(() => null);
        if (!connRes.ok) throw new Error(connData?.error || 'Failed to connect Clawn project to board');
        setResult({ machine_token: connData.machine_token, agent_id: connData?.agent?.id || selectedClawnProjectId });
        setStep(2);
        onSuccess();
        toast.success('Clawn project berhasil dikonek ke board');
        return;
      }

      const registerPayload = {
        id,
        display_name: displayName,
        role_name: roleName,
        avatar,
        provider,
        engine,
        description,
        is_native_clawn: false,
        connector: {
          connector_type: connectorType,
          transport_mode: effectiveTransportMode,
          auth_type: 'machine_token',
          endpoint_url: endpointUrl,
          base_url: baseUrl,
          agent_ref: agentRef,
          session_key: sessionKey,
          metadata: {
            runtime: 'external',
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
      <div className="w-full max-w-3xl overflow-hidden rounded-2xl border bg-white shadow-2xl dark:border-zinc-800 dark:bg-zinc-900">
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
            <div className="mb-5 flex gap-2 rounded-xl bg-zinc-100 p-1 dark:bg-zinc-800">
              <button
                type="button"
                onClick={() => setProvider('external')}
                className={`flex-1 rounded-lg px-3 py-2 text-sm font-medium ${provider === 'external' ? 'bg-white shadow dark:bg-zinc-900' : 'text-muted-foreground'}`}
              >
                Manual / External
              </button>
              <button
                type="button"
                onClick={() => setProvider('clawn')}
                className={`flex-1 rounded-lg px-3 py-2 text-sm font-medium ${provider === 'clawn' ? 'bg-white shadow dark:bg-zinc-900' : 'text-muted-foreground'}`}
              >
                From Clawn
              </button>
            </div>

            <div className="grid gap-5 lg:grid-cols-[1.2fr_0.8fr]">
              <div className="space-y-5">
                {provider === 'clawn' ? (
                  <section className="rounded-2xl border p-4 dark:border-zinc-800">
                    <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                      <FolderGit2 className="h-4 w-4 text-rose-500" />
                      Clawn Project Picker
                    </div>
                    <div className="relative mb-4">
                      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
                      <input
                        value={clawnSearch}
                        onChange={(e) => setClawnSearch(e.target.value)}
                        placeholder="Cari project / engine / capability"
                        className="w-full rounded-xl border bg-transparent py-2 pl-9 pr-3 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                      />
                    </div>

                    <div className="max-h-[420px] space-y-3 overflow-auto pr-1">
                      {clawnLoading ? (
                        <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">Loading Clawn projects...</div>
                      ) : filteredClawnProjects.length === 0 ? (
                        <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">Belum ada project Clawn yang bisa dipilih.</div>
                      ) : (
                        filteredClawnProjects.map((item) => {
                          const active = selectedClawnProjectId === item.project_id;
                          return (
                            <button
                              key={item.project_id}
                              type="button"
                              onClick={() => setSelectedClawnProjectId(item.project_id)}
                              className={`w-full rounded-2xl border p-4 text-left transition ${active ? 'border-rose-500 bg-rose-50 dark:bg-rose-900/10' : 'hover:border-zinc-400 dark:border-zinc-800 dark:hover:border-zinc-700'}`}
                            >
                              <div className="flex items-start justify-between gap-3">
                                <div>
                                  <div className="font-semibold">{item.display_name || item.project_id}</div>
                                  <div className="mt-1 text-xs text-muted-foreground">{item.project_id}</div>
                                </div>
                                <div className="flex flex-wrap gap-2 text-[10px] font-semibold uppercase tracking-wide">
                                  <span className="rounded-full bg-zinc-100 px-2 py-1 dark:bg-zinc-800">{item.engine || 'unknown'}</span>
                                  {item.plan && <span className="rounded-full bg-blue-100 px-2 py-1 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">{item.plan}</span>}
                                  <span className={`rounded-full px-2 py-1 ${item.status === 'running' ? 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-300' : 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300'}`}>{item.status || 'unknown'}</span>
                                  {item.already_connected && <span className="rounded-full bg-amber-100 px-2 py-1 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">already connected</span>}
                                </div>
                              </div>
                              <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
                                {(item.capabilities || []).slice(0, 6).map((cap) => (
                                  <span key={cap} className="rounded-full bg-zinc-100 px-2 py-1 dark:bg-zinc-800">{cap}</span>
                                ))}
                                {item.capabilities && item.capabilities.length > 6 && <span className="rounded-full bg-zinc-100 px-2 py-1 dark:bg-zinc-800">+{item.capabilities.length - 6}</span>}
                              </div>
                              {!item.is_connectable && (
                                <div className="mt-3 text-xs text-red-600 dark:text-red-400">Not connectable: {item.connect_reason || 'unknown reason'}</div>
                              )}
                            </button>
                          );
                        })
                      )}
                    </div>
                  </section>
                ) : (
                  <>
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
                            placeholder="mis. ext-ops-agent"
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
                            placeholder="mis. External Ops Agent"
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
                        <Field label="Engine">
                          <select
                            value={engine}
                            onChange={(e) => setEngine(e.target.value as Engine)}
                            className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                          >
                            <option value="webhook">Webhook Push</option>
                            <option value="rest">REST / Polling</option>
                          </select>
                        </Field>
                      </div>

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

                      {engine === 'rest' && (
                        <>
                          <Field label="Agent Ref" className="mt-4">
                            <input value={agentRef} onChange={(e) => setAgentRef(e.target.value)} className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700" />
                          </Field>
                          <Field label="Session Key" className="mt-4">
                            <input value={sessionKey} onChange={(e) => setSessionKey(e.target.value)} className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700" />
                          </Field>
                          <Field label="Base URL" className="mt-4">
                            <input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} className="w-full rounded-xl border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700" />
                          </Field>
                        </>
                      )}

                      <p className="mt-3 text-xs text-muted-foreground">{endpointHelp}</p>
                    </section>
                  </>
                )}
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
                    <div><span className="font-medium text-foreground">Transport:</span> {provider === 'clawn' ? 'auto / project-centric' : effectiveTransportMode}</div>
                    {provider === 'clawn' ? (
                      <>
                        <div><span className="font-medium text-foreground">Selected Project:</span> {selectedClawnProject?.display_name || '-'}</div>
                        <div><span className="font-medium text-foreground">Engine:</span> {selectedClawnProject?.engine || '-'}</div>
                        <div><span className="font-medium text-foreground">Status:</span> {selectedClawnProject?.status || '-'}</div>
                      </>
                    ) : (
                      <>
                        <div><span className="font-medium text-foreground">Agent ID:</span> {id || '-'}</div>
                        <div><span className="font-medium text-foreground">Display:</span> {displayName || '-'}</div>
                      </>
                    )}
                    <div><span className="font-medium text-foreground">Board Role:</span> {boardRole}</div>
                  </div>
                </section>

                {error && (
                  <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">
                    {error}
                  </div>
                )}

                <div className="flex flex-col gap-2">
                  <button
                    type="submit"
                    disabled={loading || (provider === 'clawn' && !selectedClawnProjectId)}
                    className="rounded-xl bg-black px-4 py-3 text-sm font-semibold text-white transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-white dark:text-black dark:hover:bg-zinc-200"
                  >
                    {loading ? 'Processing...' : provider === 'clawn' ? 'Connect Clawn Project' : 'Register Agent'}
                  </button>
                  <button type="button" onClick={onClose} className="rounded-xl border px-4 py-3 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800">
                    Cancel
                  </button>
                </div>
              </div>
            </div>
          </form>
        ) : (
          <div className="p-6">
            <div className="rounded-2xl border border-green-200 bg-green-50 p-5 dark:border-green-900/30 dark:bg-green-900/10">
              <div className="flex items-center gap-2 text-green-700 dark:text-green-300">
                <CheckCircle2 className="h-5 w-5" />
                <div className="text-lg font-semibold">Agent connected successfully</div>
              </div>
              <p className="mt-2 text-sm text-muted-foreground">
                Simpan machine token ini kalau runtime agent perlu call balik ke ClawnBoard runtime APIs.
              </p>
              <div className="mt-4 rounded-xl border bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950">
                <div className="mb-2 text-xs uppercase tracking-wide text-muted-foreground">Agent ID</div>
                <div className="font-mono text-sm">{result?.agent_id}</div>
                <div className="mb-2 mt-4 text-xs uppercase tracking-wide text-muted-foreground">Machine Token</div>
                <div className="break-all rounded-lg bg-zinc-100 p-3 font-mono text-xs dark:bg-zinc-900">{result?.machine_token}</div>
                <button onClick={copyToken} className="mt-3 inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800">
                  <Copy className="h-4 w-4" /> Copy token
                </button>
              </div>
              <div className="mt-5 flex gap-2">
                <button onClick={onClose} className="rounded-xl bg-black px-4 py-2 text-sm font-semibold text-white dark:bg-white dark:text-black">Done</button>
              </div>
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
      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</div>
      {children}
    </div>
  );
}
