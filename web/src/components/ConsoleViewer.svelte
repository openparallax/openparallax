<script lang="ts">
  import { afterUpdate } from 'svelte';
  import { shieldLog } from '../stores/messages';

  let filter: 'all' | 'errors' | 'shield' | 'tools' = 'all';
  let logEl: HTMLDivElement;
  let autoScroll = true;

  afterUpdate(() => {
    if (autoScroll && logEl) {
      logEl.scrollTo({ top: logEl.scrollHeight, behavior: 'smooth' });
    }
  });

  function handleScroll() {
    if (!logEl) return;
    const threshold = 50;
    autoScroll = logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight < threshold;
  }

  $: filteredLog = $shieldLog.filter(entry => {
    if (filter === 'all') return true;
    if (filter === 'errors') return entry.decision === 'BLOCK';
    if (filter === 'shield') return true;
    return true;
  });
</script>

<div class="console">
  <div class="console-header">
    <h2 class="console-title">Console</h2>
    <div class="console-filters">
      <button class="filter-btn" class:active={filter === 'all'} on:click={() => filter = 'all'}>All</button>
      <button class="filter-btn" class:active={filter === 'errors'} on:click={() => filter = 'errors'}>Errors</button>
      <button class="filter-btn" class:active={filter === 'shield'} on:click={() => filter = 'shield'}>Shield</button>
    </div>
  </div>

  <div class="console-log" bind:this={logEl} on:scroll={handleScroll}>
    {#if filteredLog.length === 0}
      <div class="empty-state">No log entries yet</div>
    {:else}
      {#each filteredLog as entry, i (i)}
        <div class="log-entry" class:error={entry.decision === 'BLOCK'}>
          <span class="log-badge" class:allow={entry.decision === 'ALLOW'} class:block={entry.decision === 'BLOCK'}>
            {entry.decision}
          </span>
          <span class="log-tool">{entry.toolName}</span>
          <span class="log-tier">Tier {entry.tier}</span>
          <span class="log-reasoning">{entry.reasoning.slice(0, 80)}</span>
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .console {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-self: stretch;
  }

  .console-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    flex-shrink: 0;
  }

  .console-title {
    font-family: 'Exo 2', sans-serif;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .console-filters {
    display: flex;
    gap: 4px;
  }

  .filter-btn {
    padding: 4px 10px;
    border-radius: 4px;
    border: 1px solid var(--accent-border);
    background: none;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    cursor: pointer;
    transition: all 150ms ease;
  }

  .filter-btn:hover { color: var(--text-secondary); }
  .filter-btn.active {
    background: var(--accent-ghost);
    color: var(--accent);
    border-color: var(--accent-border-active);
  }

  .console-log {
    flex: 1;
    overflow-y: auto;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    line-height: 1.6;
  }

  .log-entry {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 10px;
    border-bottom: 1px solid var(--accent-border);
    transition: background 150ms ease;
  }

  .log-entry:hover { background: var(--accent-ghost); }
  .log-entry.error { color: var(--error-dim); }

  .log-badge {
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
    flex-shrink: 0;
  }

  .log-badge.allow {
    background: var(--accent-subtle);
    color: var(--accent);
  }

  .log-badge.block {
    background: rgba(255, 61, 90, 0.1);
    color: var(--error);
  }

  .log-tool {
    color: var(--accent-dim);
    font-weight: 500;
    flex-shrink: 0;
  }

  .log-tier {
    color: var(--text-tertiary);
    flex-shrink: 0;
  }

  .log-reasoning {
    color: var(--text-tertiary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty-state {
    color: var(--text-tertiary);
    font-size: 14px;
    text-align: center;
    padding: 60px 0;
    font-family: 'Exo 2', sans-serif;
  }
</style>
