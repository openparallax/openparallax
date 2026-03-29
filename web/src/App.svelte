<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ArtifactCanvas from './components/ArtifactCanvas.svelte';
  import ChatPanel from './components/ChatPanel.svelte';
  import Particles from './components/Particles.svelte';
  import SettingsPanel from './components/SettingsPanel.svelte';
  import { Menu } from 'lucide-svelte';
  import { connect } from './lib/websocket';
  import { connected } from './stores/connection';
  import { currentMode } from './stores/session';
  import { sidebarOpen } from './stores/settings';
  import { getStatus } from './lib/api';

  let agentName = 'ATLAS';
  let workspace = '~/workspace';

  onMount(async () => {
    connect();
    try {
      const status = await getStatus();
      if (status.agent_name) agentName = status.agent_name.toUpperCase();
      if (status.workspace) workspace = status.workspace;
    } catch {
      /* engine not ready */
    }
  });
</script>

<div class="bg"></div>
<Particles />
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

  <div class="panels">
    <Sidebar />
    <ArtifactCanvas />
    <ChatPanel />
  </div>
</div>

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
    padding: 100px 140px;
    gap: var(--gap);
    transition: all 500ms ease;
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

  .panels {
    flex: 1;
    display: flex;
    gap: var(--gap);
    min-height: 0;
    overflow: hidden;
  }

  @media (max-width: 1200px) {
    .app { padding: 30px 40px; }
  }

  @media (max-width: 800px) {
    .app { padding: 8px; }
    .app-header { padding: 8px 12px; }
    .agent-name { font-size: 24px; }
    .agent-subtitle { display: none; }
    .hamburger { display: flex; }
  }
</style>
