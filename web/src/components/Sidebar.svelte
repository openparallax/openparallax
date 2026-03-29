<script lang="ts">
  import { onMount } from 'svelte';
  import { MessageSquare, FileText, Brain, Eye, Plus, Settings } from 'lucide-svelte';
  import { sessions, currentSessionId, currentMode } from '../stores/session';
  import { activeNavItem, settingsOpen } from '../stores/settings';
  import { clearMessages } from '../stores/messages';
  import { clearArtifactTabs } from '../stores/artifacts';
  import { listSessions, createSession, getMessages } from '../lib/api';
  import { messages } from '../stores/messages';
  import SessionList from './SessionList.svelte';
  import ShieldBadge from './ShieldBadge.svelte';

  const navItems = [
    { id: 'chat' as const, label: 'Chat', icon: MessageSquare },
    { id: 'artifacts' as const, label: 'Artifacts', icon: FileText },
    { id: 'memory' as const, label: 'Memory', icon: Brain },
    { id: 'console' as const, label: 'Console', icon: Eye },
  ];

  onMount(async () => {
    try {
      const list = await listSessions();
      sessions.set(list);
      if (list.length > 0) {
        currentSessionId.set(list[0].id);
        const msgs = await getMessages(list[0].id);
        messages.set(msgs);
      }
    } catch {
      // Server not available yet.
    }
  });

  async function handleNewSession() {
    try {
      const mode = $currentMode;
      const sess = await createSession(mode);
      sessions.update(s => [sess, ...s]);
      currentSessionId.set(sess.id);
      clearMessages();
      clearArtifactTabs();
      activeNavItem.set('chat');
    } catch {
      // Handle error silently.
    }
  }
</script>

<div class="sidebar glass">
  <div class="sidebar-header">
    <div class="sidebar-logo">&#x2B21;</div>
    <div class="sidebar-title">OpenParallax</div>
  </div>

  <nav class="sidebar-nav">
    {#each navItems as item}
      <button
        class="nav-item"
        class:active={$activeNavItem === item.id}
        on:click={() => activeNavItem.set(item.id)}
      >
        <svelte:component this={item.icon} size={16} />
        {item.label}
      </button>
    {/each}
  </nav>

  <div class="sidebar-section-title">Sessions</div>

  <div class="session-controls">
    <button class="new-session-btn" on:click={handleNewSession}>
      <Plus size={14} />
      New Session
    </button>
  </div>

  <SessionList />

  <div class="sidebar-footer">
    <ShieldBadge />
    <button class="settings-link" on:click={() => settingsOpen.set(true)}>
      <Settings size={15} />
      Settings
    </button>
  </div>
</div>

<style>
  .sidebar {
    width: 240px;
    min-width: 240px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .sidebar-header {
    padding: 18px 20px;
    border-bottom: 1px solid var(--accent-border);
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .sidebar-logo {
    width: 28px; height: 28px;
    border-radius: 6px;
    background: linear-gradient(135deg, var(--accent-subtle), var(--accent-ghost));
    border: 1px solid var(--accent-border-active);
    display: flex; align-items: center; justify-content: center;
    font-family: 'JetBrains Mono', monospace;
    font-weight: 700; font-size: 13px;
    color: var(--accent);
    box-shadow: var(--accent-glow);
  }

  .sidebar-title {
    font-family: 'JetBrains Mono', monospace;
    font-weight: 700; font-size: 14px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-primary);
  }

  .sidebar-nav {
    padding: 12px 10px;
    display: flex; flex-direction: column;
    gap: 2px;
  }

  .nav-item {
    display: flex; align-items: center;
    gap: 10px;
    padding: 10px 12px;
    border-radius: var(--radius);
    font-size: 13px; font-weight: 500;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 150ms ease;
    border: 1px solid transparent;
    background: none;
    font-family: inherit;
    width: 100%;
    text-align: left;
  }

  .nav-item:hover { background: var(--bg-surface-hover); color: var(--text-primary); }
  .nav-item.active {
    background: var(--accent-subtle);
    color: var(--accent);
    border-color: var(--accent-border);
    box-shadow: var(--accent-glow);
  }

  .sidebar-section-title {
    padding: 16px 14px 6px;
    font-size: 11px; font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-tertiary);
  }

  .session-controls {
    padding: 4px 10px;
  }

  .new-session-btn {
    display: flex; align-items: center; gap: 8px;
    width: 100%;
    padding: 8px 12px;
    border-radius: var(--radius);
    border: 1px dashed var(--accent-border);
    background: transparent;
    color: var(--accent-dim);
    font-size: 13px; font-weight: 500;
    cursor: pointer;
    transition: all 150ms ease;
    font-family: inherit;
  }

  .new-session-btn:hover {
    border-color: var(--accent-border-active);
    color: var(--accent);
    background: var(--accent-ghost);
  }

  .sidebar-footer {
    margin-top: auto;
    padding: 14px 16px;
    border-top: 1px solid var(--accent-border);
  }

  .settings-link {
    display: flex; align-items: center; gap: 8px;
    padding: 10px 12px;
    margin-top: 8px;
    border-radius: var(--radius);
    font-size: 13px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 150ms ease;
    border: none; background: none;
    font-family: inherit;
    width: 100%;
    text-align: left;
  }

  .settings-link:hover { background: var(--bg-surface-hover); color: var(--text-primary); }

  @media (max-width: 1200px) {
    .sidebar { width: 60px; min-width: 60px; }
    .sidebar-title, .sidebar-section-title, .session-controls { display: none; }
  }
</style>
