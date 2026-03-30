<script lang="ts">
  import { onMount } from 'svelte';
  import { MessageSquare, FileText, Brain, Eye, Plus, Settings, ChevronDown, RotateCw, Search } from 'lucide-svelte';
  import { sessions, currentSessionId, currentMode } from '../stores/session';
  import { activeNavItem, settingsOpen, sidebarOpen } from '../stores/settings';
  import { clearMessages, loadMessages } from '../stores/messages';
  import { clearArtifactTabs } from '../stores/artifacts';
  import { listSessions, createSession, getMessages, searchSessions } from '../lib/api';

  let sessionSearch = '';
  let searchResults: { session_id: string; title: string; match_type: string; snippet?: string }[] | null = null;
  let searchTimer: ReturnType<typeof setTimeout>;

  function handleSessionSearch() {
    clearTimeout(searchTimer);
    if (!sessionSearch.trim()) {
      searchResults = null;
      return;
    }
    searchTimer = setTimeout(async () => {
      try {
        const data = await searchSessions(sessionSearch.trim());
        searchResults = data.results || [];
      } catch {
        searchResults = [];
      }
    }, 300);
  }
  import SessionList from './SessionList.svelte';
  import ShieldBadge from './ShieldBadge.svelte';

  const navItems = [
    { id: 'chat' as const, label: 'Chat', icon: MessageSquare },
    { id: 'artifacts' as const, label: 'Artifacts', icon: FileText },
    { id: 'memory' as const, label: 'Memory', icon: Brain },
    { id: 'console' as const, label: 'Console', icon: Eye },
  ];

  let showNewSessionDropdown = false;

  onMount(async () => {
    try {
      const list = await listSessions();
      sessions.set(list);
      if (list.length > 0) {
        currentSessionId.set(list[0].id);
        const msgs = await getMessages(list[0].id);
        loadMessages(msgs);
      }
    } catch {
      // Server not available yet.
    }
  });

  async function handleNewSession(mode: 'normal' | 'otr' = 'normal') {
    showNewSessionDropdown = false;
    try {
      destroyCurrentOTR();
      const sess = await createSession(mode);
      sessions.update(s => [sess, ...s]);
      currentSessionId.set(sess.id);
      currentMode.set(mode);
      clearMessages();
      clearArtifactTabs();
      activeNavItem.set('chat');
    } catch {
      // Handle error silently.
    }
  }

  function destroyCurrentOTR() {
    if ($currentMode === 'otr' && $currentSessionId) {
      const otrId = $currentSessionId;
      sessions.update(s => s.filter(sess => sess.id !== otrId));
    }
    currentMode.set('normal');
  }

  function handleNavClick(id: typeof navItems[number]['id']) {
    activeNavItem.set(id);
    sidebarOpen.set(false);
  }

  let showRestartConfirm = false;

  async function handleRestart() {
    showRestartConfirm = false;
    try {
      await fetch('/api/restart', { method: 'POST' });
    } catch {
      /* WS will drop and reconnect */
    }
  }

  async function switchToSession(id: string) {
    if (id === $currentSessionId) return;
    const prevOTR = $currentMode === 'otr' ? $currentSessionId : null;
    const target = $sessions.find(s => s.id === id);
    currentSessionId.set(id);
    currentMode.set(target?.mode === 'otr' ? 'otr' : 'normal');
    clearMessages();
    clearArtifactTabs();
    if (prevOTR && prevOTR !== id) {
      sessions.update(s => s.filter(sess => sess.id !== prevOTR));
    }
    try {
      const msgs = await getMessages(id);
      if (msgs && msgs.length > 0) loadMessages(msgs);
    } catch { /* ignore */ }
  }

  function handleClickOutsideDropdown() {
    showNewSessionDropdown = false;
    showRestartConfirm = false;
  }
</script>

<svelte:window on:click={handleClickOutsideDropdown} />

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
    <div class="new-session-group">
      <button class="new-session-btn" on:click|stopPropagation={() => handleNewSession('normal')} title="New Session">
        <Plus size={14} />
        <span class="nav-label">New Session</span>
      </button>
      <button class="new-session-dropdown" on:click|stopPropagation={() => showNewSessionDropdown = !showNewSessionDropdown} title="Session type">
        <ChevronDown size={12} />
      </button>
    </div>
    {#if showNewSessionDropdown}
      <div class="dropdown-menu">
        <button class="dropdown-item" on:click|stopPropagation={() => handleNewSession('normal')}>Normal Session</button>
        <button class="dropdown-item otr" on:click|stopPropagation={() => handleNewSession('otr')}>OTR Session</button>
      </div>
    {/if}
  </div>

  <div class="session-search">
    <Search size={12} />
    <input
      type="text"
      class="session-search-input"
      placeholder="Search sessions..."
      bind:value={sessionSearch}
      on:input={handleSessionSearch}
    />
  </div>

  {#if searchResults !== null}
    <div class="search-results">
      {#if searchResults.length === 0}
        <div class="search-empty">No matches</div>
      {:else}
        {#each searchResults as result (result.session_id)}
          <button class="search-result" on:click={() => { switchToSession(result.session_id); sessionSearch = ''; searchResults = null; }}>
            <div class="search-result-title">{result.title || 'Untitled'}</div>
            {#if result.snippet}
              <div class="search-result-snippet">...{result.snippet}...</div>
            {/if}
          </button>
        {/each}
      {/if}
    </div>
  {:else}
    <SessionList />
  {/if}

  <div class="sidebar-footer">
    <ShieldBadge />
    <div class="footer-actions">
      <button class="settings-link" on:click={() => settingsOpen.set(true)} title="Settings">
        <Settings size={15} />
        <span class="nav-label">Settings</span>
      </button>
      <button class="restart-btn" on:click|stopPropagation={() => showRestartConfirm = !showRestartConfirm} title="Restart engine">
        <RotateCw size={13} />
      </button>
    </div>
    {#if showRestartConfirm}
      <div class="restart-confirm">
        <span>Restart engine?</span>
        <button class="confirm-yes" on:click|stopPropagation={handleRestart}>Yes</button>
        <button class="confirm-no" on:click|stopPropagation={() => showRestartConfirm = false}>No</button>
      </div>
    {/if}
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
    position: relative;
  }

  .session-search {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    margin: 4px 10px 2px;
    border-radius: var(--radius);
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
    color: var(--text-tertiary);
    flex-shrink: 0;
  }

  .session-search-input {
    flex: 1;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 12px;
    outline: none;
    font-family: inherit;
    min-width: 0;
  }
  .session-search-input::placeholder { color: var(--text-tertiary); }

  .search-results {
    flex: 1;
    overflow-y: auto;
    padding: 4px 10px;
  }

  .search-empty {
    color: var(--text-tertiary);
    font-size: 12px;
    text-align: center;
    padding: 20px 0;
  }

  .search-result {
    width: 100%;
    padding: 8px 10px;
    border: none;
    background: none;
    text-align: left;
    font-family: inherit;
    color: var(--text-primary);
    cursor: pointer;
    border-radius: var(--radius);
    transition: background 150ms ease;
    margin-bottom: 2px;
  }
  .search-result:hover { background: var(--bg-surface-hover); }

  .search-result-title {
    font-size: 13px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .search-result-snippet {
    font-size: 11px;
    color: var(--text-tertiary);
    margin-top: 2px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .new-session-group {
    display: flex;
    gap: 0;
  }

  .new-session-btn {
    display: flex; align-items: center; gap: 8px;
    flex: 1;
    padding: 8px 12px;
    border-radius: var(--radius) 0 0 var(--radius);
    border: 1px dashed var(--accent-border);
    border-right: none;
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

  .new-session-dropdown {
    display: flex; align-items: center; justify-content: center;
    width: 28px;
    border-radius: 0 var(--radius) var(--radius) 0;
    border: 1px dashed var(--accent-border);
    background: transparent;
    color: var(--accent-dim);
    cursor: pointer;
    transition: all 150ms ease;
  }

  .new-session-dropdown:hover {
    border-color: var(--accent-border-active);
    color: var(--accent);
    background: var(--accent-ghost);
  }

  .dropdown-menu {
    position: absolute;
    left: 10px; right: 10px;
    top: 44px;
    background: rgba(12, 16, 28, 0.95);
    border: 1px solid var(--accent-border-active);
    border-radius: var(--radius);
    overflow: hidden;
    z-index: 20;
    backdrop-filter: blur(16px);
  }

  .dropdown-item {
    width: 100%;
    padding: 8px 14px;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 13px;
    font-family: inherit;
    cursor: pointer;
    text-align: left;
    transition: background 150ms ease;
  }

  .dropdown-item:hover { background: var(--accent-ghost); }

  .dropdown-item.otr {
    color: var(--warning);
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

  .footer-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .restart-btn {
    width: 32px; height: 32px;
    border-radius: var(--radius);
    border: 1px solid var(--accent-border);
    background: none;
    color: var(--text-tertiary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 150ms ease;
    flex-shrink: 0;
  }
  .restart-btn:hover {
    color: var(--warning);
    border-color: rgba(255, 171, 0, 0.3);
    background: rgba(255, 171, 0, 0.05);
  }

  .restart-confirm {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    margin-top: 6px;
    font-size: 12px;
    color: var(--warning);
    background: rgba(255, 171, 0, 0.05);
    border: 1px solid rgba(255, 171, 0, 0.15);
    border-radius: var(--radius);
  }
  .restart-confirm span { flex: 1; }
  .confirm-yes, .confirm-no {
    padding: 2px 10px;
    border-radius: 4px;
    border: none;
    font-size: 11px;
    font-family: 'JetBrains Mono', monospace;
    cursor: pointer;
  }
  .confirm-yes { background: var(--warning); color: var(--bg-void); }
  .confirm-no { background: var(--accent-ghost); color: var(--text-secondary); }

  .sidebar-backdrop {
    display: none;
  }

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
    .session-search { display: none; }
    .sidebar-footer { padding: 10px 8px; }
    .settings-link { padding: 10px; justify-content: center; }
    .settings-link .nav-label { display: none; }
    .footer-actions { justify-content: center; }
    .restart-btn { display: none; }
    .restart-confirm { display: none; }
  }

  :global(.session-list) {
    display: block;
  }
  @media (max-width: 1200px) {
    :global(.session-list) {
      display: none;
    }
  }
  @media (max-width: 800px) {
    :global(.session-list) {
      display: block;
    }
  }

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
