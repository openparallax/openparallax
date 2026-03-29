<script lang="ts">
  import { ChevronRight } from 'lucide-svelte';
  import { activeDetailTab } from '../stores/settings';
  import { artifacts } from '../stores/messages';
  import ArtifactCard from './ArtifactCard.svelte';
  import ShieldLog from './ShieldLog.svelte';
  import MemoryPanel from './MemoryPanel.svelte';

  const tabs = [
    { id: 'artifacts' as const, label: 'Artifacts' },
    { id: 'shield' as const, label: 'Shield' },
    { id: 'memory' as const, label: 'Memory' },
  ];
</script>

<div class="detail-panel glass">
  <div class="detail-header">
    <div class="detail-title">Context</div>
    <button class="collapse-btn"><ChevronRight size={14} /></button>
  </div>

  <div class="detail-tabs">
    {#each tabs as tab}
      <button
        class="detail-tab"
        class:active={$activeDetailTab === tab.id}
        on:click={() => activeDetailTab.set(tab.id)}
      >
        {tab.label}
      </button>
    {/each}
  </div>

  <div class="detail-content">
    {#if $activeDetailTab === 'artifacts'}
      {#if $artifacts.length === 0}
        <div class="empty-state">No artifacts yet</div>
      {:else}
        {#each $artifacts as artifact (artifact.id)}
          <ArtifactCard {artifact} />
        {/each}
      {/if}

    {:else if $activeDetailTab === 'shield'}
      <ShieldLog />

    {:else if $activeDetailTab === 'memory'}
      <MemoryPanel />
    {/if}
  </div>
</div>

<style>
  .detail-panel {
    width: 340px; min-width: 340px;
    display: flex; flex-direction: column;
    overflow: hidden;
  }

  .detail-header {
    padding: 14px 18px;
    border-bottom: 1px solid var(--accent-border);
    display: flex; align-items: center;
    justify-content: space-between;
  }

  .detail-title {
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px; font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-secondary);
  }

  .collapse-btn {
    width: 28px; height: 28px;
    border-radius: var(--radius);
    border: 1px solid var(--accent-border);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 150ms ease;
  }

  .collapse-btn:hover {
    border-color: var(--accent-border-active);
    color: var(--text-primary);
  }

  .detail-tabs {
    display: flex;
    padding: 0 14px;
    border-bottom: 1px solid var(--accent-border);
  }

  .detail-tab {
    padding: 10px 14px;
    font-size: 12px; font-weight: 500;
    color: var(--text-tertiary);
    cursor: pointer;
    border: none; background: none;
    border-bottom: 2px solid transparent;
    transition: all 150ms ease;
    font-family: inherit;
  }

  .detail-tab:hover { color: var(--text-secondary); }
  .detail-tab.active { color: var(--accent); border-bottom-color: var(--accent); }

  .detail-content {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
  }

  .empty-state {
    color: var(--text-tertiary);
    font-size: 13px;
    text-align: center;
    padding: 40px 0;
  }

  @media (max-width: 1200px) {
    .detail-panel { display: none; }
  }
</style>
