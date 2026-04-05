<script lang="ts">
  import ConsoleMetrics from './ConsoleMetrics.svelte';
  import ConsoleLive from './ConsoleLive.svelte';
  import ConsoleAudit from './ConsoleAudit.svelte';

  type Tab = 'metrics' | 'live' | 'audit';
  let activeTab: Tab = 'metrics';
</script>

<div class="console">
  {#if activeTab === 'metrics'}
    <ConsoleMetrics />
  {:else if activeTab === 'live'}
    <ConsoleLive />
  {:else}
    <ConsoleAudit />
  {/if}

  <div class="console-tabs">
    <button class="console-tab" class:active={activeTab === 'metrics'} on:click={() => { activeTab = 'metrics'; }}>Metrics</button>
    <button class="console-tab" class:active={activeTab === 'audit'} on:click={() => { activeTab = 'audit'; }}>Audit Trail</button>
    <button class="console-tab" class:active={activeTab === 'live'} on:click={() => { activeTab = 'live'; }}>Live Log</button>
  </div>
</div>

<style>
  .console {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .console-tabs {
    display: flex;
    gap: 0;
    border-top: 1px solid var(--accent-border);
    flex-shrink: 0;
    margin-top: 8px;
  }

  .console-tab {
    flex: 1;
    padding: 8px;
    border: none;
    background: none;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    cursor: pointer;
    text-align: center;
    transition: all 150ms ease;
    border-bottom: 2px solid transparent;
  }
  .console-tab:hover { color: var(--text-secondary); }
  .console-tab.active { color: var(--accent); border-bottom-color: var(--accent); }
</style>
