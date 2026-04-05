<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ChatPanel from './components/ChatPanel.svelte';
  import ConsoleViewer from './components/ConsoleViewer.svelte';
  import Particles from './components/Particles.svelte';
  import SettingsPanel from './components/SettingsPanel.svelte';
  import SetupWizard from './components/SetupWizard.svelte';
  import { Menu } from 'lucide-svelte';
  import { connect } from './lib/websocket';
  import { connected, reconnecting } from './stores/connection';
  import { sessions, currentSessionId, currentMode } from './stores/session';
  import { sidebarOpen, settingsOpen, activeNavItem } from './stores/settings';
  import { clearMessages, addSystemMessage, messages } from './stores/messages';
  import { createSession, getStatus } from './lib/api';

  let agentName = 'ATLAS';
  let workspace = '~/workspace';
  let originalTitle = 'OpenParallax';
  let setupRequired = false;

  let sidebarWidth = parseInt(localStorage.getItem('op_sidebar_w') || '240');
  let resizing: 'sidebar' | null = null;

  function startSidebarResize(e: MouseEvent) {
    e.preventDefault();
    resizing = 'sidebar';
    const startX = e.clientX;
    const startW = sidebarWidth;

    function onMove(ev: MouseEvent) {
      sidebarWidth = Math.max(180, Math.min(320, startW + (ev.clientX - startX)));
    }

    function onUp() {
      resizing = null;
      localStorage.setItem('op_sidebar_w', String(sidebarWidth));
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    }

    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  onMount(async () => {
    originalTitle = document.title;

    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) {
        document.title = originalTitle;
      }
    });

    try {
      const status = await getStatus();
      if ((status as any).setup_required) {
        setupRequired = true;
        return;
      }
      if (status.agent_name) agentName = status.agent_name.toUpperCase();
      if (status.workspace) workspace = status.workspace;
    } catch {
      /* engine not ready */
    }

    connect();
  });

  function handleSetupComplete() {
    setupRequired = false;
    window.location.reload();
  }

  function handleKeydown(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey;
    if (mod && e.key === 'n' && !e.shiftKey) {
      e.preventDefault();
      createNewSession('normal');
    } else if (mod && e.shiftKey && e.key === 'O') {
      e.preventDefault();
      createNewSession('otr');
    } else if (mod && e.key === 'l') {
      e.preventDefault();
      messages.set([]);
      addSystemMessage('Chat cleared. History is preserved.');
    } else if (e.key === 'Escape') {
      settingsOpen.set(false);
      sidebarOpen.set(false);
    }
  }

  async function createNewSession(mode: 'normal' | 'otr') {
    try {
      if ($currentMode === 'otr' && $currentSessionId) {
        const otrId = $currentSessionId;
        sessions.update(s => s.filter(sess => sess.id !== otrId));
      }
      const sess = await createSession(mode);
      sessions.update(s => [sess, ...s]);
      currentSessionId.set(sess.id);
      currentMode.set(mode);
      clearMessages();
    } catch {
      /* ignore */
    }
  }

  $: showChat = $activeNavItem === 'chat';
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="bg"></div>
<Particles />

{#if setupRequired}
  <SetupWizard on:complete={handleSetupComplete} />
{:else}
<SettingsPanel />

<div class="app" class:otr={$currentMode === 'otr'}>
  <header class="app-header">
    <div class="header-left">
      <button class="hamburger" on:click={() => sidebarOpen.update(v => !v)} aria-label="Toggle sidebar">
        <Menu size={20} />
      </button>
      <span class="agent-name">{agentName}</span>
      <span class="agent-subtitle">{workspace}</span>
    </div>
    <div class="header-status">
      {#if $currentMode === 'otr'}
        <span class="status-badge otr-badge">&#x1F512; Private</span>
      {:else if $connected}
        <span class="status-badge live">SYNC_LIVE</span>
      {:else}
        <span class="status-badge offline">OFFLINE</span>
      {/if}
    </div>
  </header>

  {#if $reconnecting}
    <div class="reconnecting-bar">Reconnecting...</div>
  {/if}

  <div class="panels" class:resizing={resizing !== null}>
    <div class="sidebar-wrap" style="--sw:{sidebarWidth}px">
      <Sidebar />
    </div>
    <div class="resize-handle" on:mousedown={startSidebarResize} role="separator" aria-label="Resize sidebar"></div>

    <div class="main-area">
      {#if showChat}
        <ChatPanel />
      {:else if $activeNavItem === 'console'}
        <div class="alt-view glass">
          <ConsoleViewer />
        </div>
      {/if}
    </div>
  </div>
</div>
{/if}

<style>
  .bg {
    position: fixed;
    inset: 0;
    z-index: 0;
    background:
      radial-gradient(ellipse at 50% 30%, rgba(0, 220, 255, 0.07) 0%, transparent 50%),
      radial-gradient(ellipse at 10% 85%, rgba(100, 40, 160, 0.06) 0%, transparent 40%),
      radial-gradient(ellipse at 90% 50%, rgba(0, 200, 160, 0.05) 0%, transparent 35%),
      radial-gradient(ellipse at 60% 80%, rgba(0, 120, 255, 0.04) 0%, transparent 30%),
      var(--bg-void);
    pointer-events: none;
  }

  .app {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 60px 80px;
    gap: var(--gap);
    transition: padding 300ms ease;
  }

  .app-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    padding: 0 4px;
    min-height: 36px;
  }

  .header-left {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }

  .hamburger {
    display: none;
    align-items: center;
    justify-content: center;
    width: 36px; height: 36px;
    border: 1px solid var(--accent-border);
    border-radius: var(--radius);
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 150ms ease;
    flex-shrink: 0;
    align-self: center;
  }
  .hamburger:hover {
    color: var(--text-primary);
    border-color: var(--accent-border-active);
    background: var(--accent-ghost);
  }

  .agent-name {
    font-family: 'Exo 2', sans-serif;
    font-weight: 800;
    font-size: 36px;
    letter-spacing: 0.04em;
    color: var(--text-primary);
    text-transform: uppercase;
  }

  .agent-subtitle {
    font-size: 13px;
    color: var(--text-tertiary);
    margin-left: 14px;
    font-weight: 400;
  }

  .header-status {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .status-badge {
    padding: 4px 14px;
    border-radius: 20px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.05em;
  }

  .status-badge.live {
    background: rgba(0, 230, 118, 0.1);
    color: var(--success);
    border: 1px solid rgba(0, 230, 118, 0.2);
  }

  .status-badge.offline {
    background: rgba(255, 61, 90, 0.1);
    color: var(--error);
    border: 1px solid rgba(255, 61, 90, 0.2);
  }

  .status-badge.otr-badge {
    background: rgba(255, 171, 0, 0.1);
    color: var(--warning);
    border: 1px solid rgba(255, 171, 0, 0.2);
  }

  .reconnecting-bar {
    padding: 4px 0;
    text-align: center;
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: var(--warning);
    background: rgba(255, 171, 0, 0.08);
    border: 1px solid rgba(255, 171, 0, 0.15);
    border-radius: 4px;
    animation: pulse-bar 2s ease-in-out infinite;
  }

  @keyframes pulse-bar {
    0%, 100% { opacity: 0.7; }
    50% { opacity: 1; }
  }

  .panels {
    flex: 1;
    display: flex;
    gap: 0;
    min-height: 0;
    overflow: hidden;
  }

  .panels.resizing {
    cursor: col-resize;
    user-select: none;
  }

  .sidebar-wrap {
    width: var(--sw, 240px);
    min-width: var(--sw, 240px);
    flex-shrink: 0;
  }

  .main-area {
    flex: 1;
    display: flex;
    min-width: 0;
    gap: 0;
  }

  .alt-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 20px;
  }

  .resize-handle {
    width: var(--gap);
    cursor: col-resize;
    flex-shrink: 0;
    position: relative;
  }

  .resize-handle::after {
    content: '';
    position: absolute;
    top: 0; bottom: 0;
    left: 50%;
    width: 1px;
    background: transparent;
    transition: background 200ms ease;
  }

  .resize-handle:hover::after {
    background: var(--accent-border-active);
  }

  @media (max-width: 1200px) {
    .app { padding: 30px 40px; }
    .sidebar-wrap { width: 56px !important; min-width: 56px !important; }
  }

  @media (max-width: 800px) {
    .app { padding: 8px; }
    .app-header { padding: 8px 12px; }
    .agent-name { font-size: 24px; }
    .agent-subtitle { display: none; }
    .hamburger { display: flex; }
    .sidebar-wrap { width: 0 !important; min-width: 0 !important; overflow: visible; }
    .resize-handle { display: none; }
  }

  @media (max-width: 480px) {
    .app { padding: 4px; }
    .app-header { padding: 4px 8px; }
    .agent-name { font-size: 20px; }
    .status-badge { padding: 3px 8px; font-size: 10px; }
  }
</style>
