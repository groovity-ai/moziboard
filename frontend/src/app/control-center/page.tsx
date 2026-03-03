'use client';

import React, { useState, useEffect } from 'react';
import useSWR, { mutate } from 'swr';
import { 
  Shield, 
  Key, 
  User, 
  Settings, 
  RefreshCw, 
  Save, 
  Check,
  XCircle
} from 'lucide-react';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

// --- Types ---
interface AuthProfile {
  provider: string;
  mode: 'oauth' | 'api_key';
  email?: string;
  key?: string;
}

interface AgentIdentity {
  name: string;
  emoji: string;
}

interface Agent {
  id: string;
  model: string;
  default?: boolean;
  identity?: AgentIdentity;
  [key: string]: any; // Allow other properties
}

interface OpenClawConfig {
  auth: {
    profiles: Record<string, AuthProfile>;
    order: string[];
  };
  agents: {
    defaults: any;
    list: Agent[];
  };
  [key: string]: any;
}

interface ManagerProps {
  config: OpenClawConfig;
  onSave: (newConfig: OpenClawConfig) => Promise<void>;
  isSaving: boolean;
}

// --- Main Component ---
export default function ControlCenter() {
  const { data: config, error } = useSWR<OpenClawConfig>('/api/openclaw/config', fetcher);
  const [isSaving, setIsSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<'auth' | 'agents' | 'system'>('auth');
  const [saveStatus, setSaveStatus] = useState<'idle' | 'success' | 'error'>('idle');

  if (error) return <div className="p-8 text-red-500">Failed to load config. Is Backend running?</div>;
  if (!config) return <div className="p-8">Loading Agent Nexus...</div>;

  const handleSave = async (newConfig: OpenClawConfig) => {
    setIsSaving(true);
    setSaveStatus('idle');
    try {
      const res = await fetch('/api/openclaw/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newConfig),
      });
      if (!res.ok) throw new Error('Failed to save');
      await mutate('/api/openclaw/config'); // Revalidate
      setSaveStatus('success');
      setTimeout(() => setSaveStatus('idle'), 3000);
    } catch (e) {
      setSaveStatus('error');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="flex h-screen w-full bg-gray-50 dark:bg-zinc-900 text-gray-900 dark:text-gray-100">
      {/* Sidebar */}
      <div className="w-64 border-r bg-white p-4 dark:bg-zinc-950 dark:border-zinc-800">
        <h1 className="mb-8 flex items-center gap-2 text-xl font-bold text-rose-500">
          <Shield size={24} /> Agent Nexus
        </h1>
        
        <nav className="space-y-2">
          <SidebarButton 
            active={activeTab === 'auth'} 
            onClick={() => setActiveTab('auth')} 
            icon={<Key size={18} />} 
            label="Auth Profiles" 
          />
          <SidebarButton 
            active={activeTab === 'agents'} 
            onClick={() => setActiveTab('agents')} 
            icon={<User size={18} />} 
            label="Agents" 
          />
          <SidebarButton 
            active={activeTab === 'system'} 
            onClick={() => setActiveTab('system')} 
            icon={<Settings size={18} />} 
            label="System" 
          />
        </nav>
      </div>

      {/* Main Content */}
      <div className="flex-1 overflow-y-auto p-8">
        <div className="mx-auto max-w-4xl">
          {/* Header Status */}
          {saveStatus !== 'idle' && (
            <div className={`mb-4 flex items-center gap-2 rounded-lg p-3 text-sm font-medium ${
              saveStatus === 'success' 
                ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' 
                : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
            }`}>
              {saveStatus === 'success' ? <Check size={18} /> : <XCircle size={18} />}
              {saveStatus === 'success' ? 'Configuration saved successfully!' : 'Failed to save configuration.'}
            </div>
          )}

          {activeTab === 'auth' && (
            <AuthManager config={config} onSave={handleSave} isSaving={isSaving} />
          )}
          {activeTab === 'agents' && (
            <AgentManager config={config} onSave={handleSave} isSaving={isSaving} />
          )}
          {activeTab === 'system' && (
            <SystemManager config={config} onSave={handleSave} isSaving={isSaving} />
          )}
        </div>
      </div>
    </div>
  );
}

function SidebarButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: React.ReactNode; label: string }) {
  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-3 rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
        active 
          ? 'bg-rose-50 text-rose-600 dark:bg-rose-500/10 dark:text-rose-400' 
          : 'hover:bg-gray-100 dark:hover:bg-zinc-800'
      }`}
    >
      {icon} {label}
    </button>
  );
}

function SystemManager({ config, onSave, isSaving }: ManagerProps) {
  const { data: soulData, mutate: mutateSoul } = useSWR('/api/openclaw/soul', fetcher);
  const [soulContent, setSoulContent] = useState('');
  const [isRestarting, setIsRestarting] = useState(false);
  const [soulStatus, setSoulStatus] = useState<'idle' | 'saved' | 'error'>('idle');

  useEffect(() => {
    if (soulData?.content) setSoulContent(soulData.content);
  }, [soulData]);

  const handleSoulSave = async () => {
    try {
      await fetch('/api/openclaw/soul', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: soulContent }),
      });
      setSoulStatus('saved');
      setTimeout(() => setSoulStatus('idle'), 2000);
      mutateSoul();
    } catch (e) {
      setSoulStatus('error');
    }
  };

  const handleRestart = async () => {
    if (!confirm('Are you sure you want to restart OpenClaw? Service will be briefly unavailable.')) return;
    setIsRestarting(true);
    try {
      await fetch('/api/openclaw/restart', { method: 'POST' });
    } catch (e) {
      // Ignore error as restart might kill the connection
    } finally {
      setTimeout(() => setIsRestarting(false), 2000); // Simulate restart delay
    }
  };

  return (
    <div className="space-y-8">
      {/* System Actions */}
      <div className="rounded-xl border bg-white p-6 shadow-sm dark:bg-zinc-900 dark:border-zinc-800">
        <h2 className="mb-4 text-xl font-bold text-rose-500 flex items-center gap-2">
          <Settings size={20} /> System Controls
        </h2>
        <div className="flex items-center justify-between rounded-lg bg-gray-50 p-4 dark:bg-zinc-800">
          <div>
            <h3 className="font-bold">Restart Gateway</h3>
            <p className="text-sm text-gray-500">Apply config changes and restart services.</p>
          </div>
          <button
            onClick={handleRestart}
            disabled={isRestarting}
            className="flex items-center gap-2 rounded-lg bg-red-500 px-4 py-2 text-white hover:bg-red-600 disabled:opacity-50 transition-all"
          >
            <RefreshCw size={18} className={isRestarting ? "animate-spin" : ""} />
            {isRestarting ? 'Restarting...' : 'Restart Now'}
          </button>
        </div>
      </div>

      {/* SOUL Editor */}
      <div className="rounded-xl border bg-white p-6 shadow-sm dark:bg-zinc-900 dark:border-zinc-800">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-bold text-rose-500 flex items-center gap-2">
            <User size={20} /> Personality Core (SOUL.md)
          </h2>
          <button
            onClick={handleSoulSave}
            className={`flex items-center gap-2 rounded-lg px-4 py-2 text-white transition-all ${
              soulStatus === 'saved' ? 'bg-green-600' : 'bg-black hover:bg-gray-800 dark:bg-white dark:text-black'
            }`}
          >
            {soulStatus === 'saved' ? <Check size={18} /> : <Save size={18} />}
            {soulStatus === 'saved' ? 'Saved!' : 'Save SOUL'}
          </button>
        </div>
        <textarea
          value={soulContent}
          onChange={(e) => setSoulContent(e.target.value)}
          className="h-[400px] w-full rounded-lg border bg-gray-50 p-4 font-mono text-sm outline-none focus:border-rose-500 dark:bg-zinc-950 dark:border-zinc-700 dark:text-gray-300"
          spellCheck={false}
        />
      </div>
    </div>
  );
}

function AuthManager({ config, onSave, isSaving }: ManagerProps) {
  const [profiles, setProfiles] = useState<Record<string, AuthProfile>>(config.auth?.profiles || {});

  // Sync state when config changes (e.g. background update)
  useEffect(() => {
    if (config.auth?.profiles) {
      setProfiles(config.auth.profiles);
    }
  }, [config]);

  const updateProfile = (key: string, field: keyof AuthProfile, value: string) => {
    const newProfiles = { 
      ...profiles, 
      [key]: { ...profiles[key], [field]: value } 
    };
    setProfiles(newProfiles);
  };

  const saveChanges = () => {
    onSave({ ...config, auth: { ...config.auth, profiles } });
  };

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h2 className="text-2xl font-bold">Authentication Profiles</h2>
        <button
          onClick={saveChanges}
          disabled={isSaving}
          className="flex items-center gap-2 rounded-lg bg-black px-4 py-2 text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-white dark:text-black"
        >
          <Save size={18} /> {isSaving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>

      <div className="grid gap-6">
        {Object.entries(profiles).map(([key, profile]) => (
          <div key={key} className="rounded-xl border bg-white p-6 shadow-sm dark:bg-zinc-900 dark:border-zinc-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="font-mono text-lg font-bold text-rose-500">{key}</h3>
              <span className={`rounded-full px-2 py-1 text-xs font-bold ${
                profile.mode === 'oauth' ? 'bg-blue-100 text-blue-700' : 'bg-green-100 text-green-700'
              }`}>
                {profile.mode.toUpperCase()}
              </span>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Provider</label>
                <input
                  type="text"
                  value={profile.provider}
                  readOnly
                  className="w-full rounded-md border bg-gray-50 px-3 py-2 text-sm text-gray-500 outline-none dark:bg-zinc-800 dark:border-zinc-700"
                />
              </div>
              
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Email / ID</label>
                <input
                  type="text"
                  value={profile.email || ''}
                  onChange={(e) => updateProfile(key, 'email', e.target.value)}
                  className="w-full rounded-md border px-3 py-2 text-sm outline-none focus:border-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                />
              </div>

              {profile.mode === 'api_key' && (
                <div className="md:col-span-2">
                  <label className="mb-1 block text-xs font-medium text-gray-500">API Key</label>
                  <div className="relative">
                    <input
                      type="password"
                      value={profile.key || ''}
                      onChange={(e) => updateProfile(key, 'key', e.target.value)}
                      className="w-full rounded-md border px-3 py-2 pr-10 text-sm font-mono outline-none focus:border-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                      placeholder="sk-..."
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function AgentManager({ config, onSave, isSaving }: ManagerProps) {
  const [agents, setAgents] = useState<Agent[]>(config.agents?.list || []);

  // Sync state
  useEffect(() => {
    if (config.agents?.list) {
      setAgents(config.agents.list);
    }
  }, [config]);

  const updateAgent = (index: number, field: string, value: any) => {
    const newAgents = [...agents];
    if (field.includes('.')) {
      const [parent, child] = field.split('.');
      if (newAgents[index][parent]) {
         newAgents[index][parent] = { ...newAgents[index][parent], [child]: value };
      }
    } else {
      newAgents[index][field] = value;
    }
    setAgents(newAgents);
  };

  const saveChanges = () => {
    onSave({ ...config, agents: { ...config.agents, list: agents } });
  };

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h2 className="text-2xl font-bold">Agent Configuration</h2>
        <button
          onClick={saveChanges}
          disabled={isSaving}
          className="flex items-center gap-2 rounded-lg bg-black px-4 py-2 text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-white dark:text-black"
        >
          <Save size={18} /> {isSaving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>

      <div className="grid gap-6">
        {agents.map((agent: Agent, index: number) => (
          <div key={agent.id} className="rounded-xl border bg-white p-6 shadow-sm dark:bg-zinc-900 dark:border-zinc-800">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gray-100 text-xl dark:bg-zinc-800">
                  {agent.identity?.emoji || '🤖'}
                </div>
                <div>
                  <h3 className="font-bold">{agent.id}</h3>
                  <p className="text-xs text-gray-500">{agent.identity?.name || agent.id}</p>
                </div>
              </div>
              <div className="flex gap-2">
                {agent.default && (
                  <span className="rounded-full bg-blue-100 px-2 py-1 text-xs font-bold text-blue-700">DEFAULT</span>
                )}
                <span className="rounded-full bg-gray-100 px-2 py-1 text-xs font-mono text-gray-600 dark:bg-zinc-800 dark:text-gray-400">
                  {agent.model}
                </span>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Agent ID</label>
                <input
                  type="text"
                  value={agent.id}
                  readOnly
                  className="w-full rounded-md border bg-gray-50 px-3 py-2 text-sm text-gray-500 outline-none dark:bg-zinc-800 dark:border-zinc-700"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Model</label>
                <input
                  type="text"
                  value={agent.model}
                  onChange={(e) => updateAgent(index, 'model', e.target.value)}
                  className="w-full rounded-md border px-3 py-2 text-sm outline-none focus:border-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Display Name</label>
                <input
                  type="text"
                  value={agent.identity?.name || ''}
                  onChange={(e) => updateAgent(index, 'identity.name', e.target.value)}
                  className="w-full rounded-md border px-3 py-2 text-sm outline-none focus:border-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-500">Emoji</label>
                <input
                  type="text"
                  value={agent.identity?.emoji || ''}
                  onChange={(e) => updateAgent(index, 'identity.emoji', e.target.value)}
                  className="w-full rounded-md border px-3 py-2 text-sm outline-none focus:border-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                />
              </div>
            </div>
          </div>
        ))}
        
        <button className="flex w-full items-center justify-center rounded-xl border border-dashed border-gray-300 p-4 text-sm font-medium text-gray-500 hover:border-rose-500 hover:text-rose-500 dark:border-zinc-700">
          + Add New Agent (Coming Soon)
        </button>
      </div>
    </div>
  );
}
