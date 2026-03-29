<script lang="ts">
  import { onMount } from 'svelte';
  import { MessageSquare, FileText, Brain, Eye, Plus, Settings } from 'lucide-svelte';
  import { sessions, currentSessionId, currentMode } from '../stores/session';
  import { activeNavItem, settingsOpen, sidebarOpen } from '../stores/settings';
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

  function handleNavClick(id: typeof navItems[number]['id']) {
    activeNavItem.set(id);
    sidebarOpen.set(false);
  }
</script>

<div class="sidebar glass" class:mobile-open={$sidebarOpen}>
  <div class="sidebar-header">
    <div class="sidebar-logo">&#x2B21;</div>
    <span class="sidebar-title">OpenParallax</span>
  </div>

  <nav class="sidebar-nav">
    {#each navItems as item}
      <button
        class="nav-item"
        class:active={$activeNavItem === item.id}
        on:click={() => handleNavClick(item.id)}
        title={item.label}
      >
        <svelte:component this={item.icon} size={16} />
        <span class="nav-label">{item.label}</span>
      </button>
    {/each}
  </nav>

  <div class="sidebar-section-title">Sessions</div>

  <div class="session-controls">
    <button class="new-session-btn" on:click={handleNewSession} title="New Session">
      <Plus size={14} />
      <span class="nav-label">New Session</span>
    </button>
  </div>

  <SessionList />

  <div class="sidebar-footer">
    <ShieldBadge />
    <button class="settings-link" on:click={() => settingsOpen.set(true)} title="Settings">
      <Settings size={15} />
      <span class="nav-label">Settings</span>
    </button>
  </div>
</div>

{#if $sidebarOpen}
  <button class="sidebar-backdrop" on:click={() => sidebarOpen.set(false)} aria-label="Close sidebar"></button>
{/if}

<style>
  .sidebar {
    width: 240px;
    min-width: 240px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: width 200ms ease, min-width 200ms ease;
  }

  .sidebar-header {
    padding: 18px 20px;
    border-bottom: 1px solid var(--accent-border);
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }

  .sidebar-logo {
    width: 28px; height: 28px;
    min-width: 28px;
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
    white-space: nowrap;
    overflow: hidden;
  }

  .sidebar-nav {
    padding: 12px 10px;
    display: flex; flex-direction: column;
    gap: 2px;
    flex-shrink: 0;
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
    white-space: nowrap;
    overflow: hidden;
  }

  .nav-item:hover { background: var(--bg-surface-hover); color: var(--text-primary); }
  .nav-item.active {
    background: var(--accent-subtle);
    color: var(--accent);
    border-color: var(--accent-border);
    box-shadow: var(--accent-glow);
  }

  .nav-label {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sidebar-section-title {
    padding: 16px 14px 6px;
    font-size: 11px; font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-tertiary);
    flex-shrink: 0;
  }

  .session-controls {
    padding: 4px 10px;
    flex-shrink: 0;
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
    white-space: nowrap;
    overflow: hidden;
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
    flex-shrink: 0;
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
    white-space: nowrap;
    overflow: hidden;
  }

  .settings-link:hover { background: var(--bg-surface-hover); color: var(--text-primary); }

  .sidebar-backdrop {
    display: none;
  }

  /* Medium: icon-only strip */
  @media (max-width: 1200px) {
    .sidebar {
      width: 56px;
      min-width: 56px;
    }
    .sidebar-header { padding: 14px 0; justify-content: center; }
    .sidebar-title { display: none; }
    .sidebar-nav { padding: 8px 6px; }
    .nav-item { padding: 10px; justify-content: center; }
    .nav-label { display: none; }
    .sidebar-section-title { display: none; }
    .session-controls { display: none; }
    .sidebar-footer { padding: 10px 8px; }
    .settings-link { padding: 10px; justify-content: center; }
    .settings-link .nav-label { display: none; }
  }

  /* Small: fully hidden, slide-over when open */
  @media (max-width: 800px) {
    .sidebar {
      position: fixed;
      left: 0; top: 0; bottom: 0;
      width: 260px;
      min-width: 260px;
      z-index: 50;
      transform: translateX(-100%);
      transition: transform 300ms ease;
      border-radius: 0 var(--radius) var(--radius) 0;
    }
    .sidebar.mobile-open {
      transform: translateX(0);
    }
    .sidebar-header { padding: 18px 20px; justify-content: flex-start; }
    .sidebar-title { display: inline; }
    .sidebar-nav { padding: 12px 10px; }
    .nav-item { padding: 10px 12px; justify-content: flex-start; }
    .nav-label { display: inline; }
    .sidebar-section-title { display: block; }
    .session-controls { display: block; }
    .settings-link { justify-content: flex-start; }
    .settings-link .nav-label { display: inline; }

    .sidebar-backdrop {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 49;
      background: rgba(0, 0, 0, 0.5);
      border: none;
      cursor: default;
    }
  }
</style>
