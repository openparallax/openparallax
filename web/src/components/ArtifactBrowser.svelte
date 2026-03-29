<script lang="ts">
  import { onMount } from 'svelte';
  import { listArtifacts } from '../lib/api';
  import { artifacts as liveArtifacts } from '../stores/messages';
  import { openArtifactTab } from '../stores/artifacts';
  import { activeNavItem } from '../stores/settings';
  import type { Artifact } from '../lib/types';

  let dbArtifacts: Artifact[] = [];
  let loading = true;

  onMount(async () => {
    try {
      const fetched = await listArtifacts();
      dbArtifacts = Array.isArray(fetched) ? fetched : [];
    } catch {
      dbArtifacts = [];
    }
    loading = false;
  });

  $: allArtifacts = mergeArtifacts(dbArtifacts, $liveArtifacts);

  function mergeArtifacts(db: Artifact[], live: Artifact[]): Artifact[] {
    const seen = new Set<string>();
    const merged: Artifact[] = [];
    for (const a of live) {
      if (!seen.has(a.id)) {
        seen.add(a.id);
        merged.push(a);
      }
    }
    for (const a of db) {
      if (!seen.has(a.id)) {
        seen.add(a.id);
        merged.push(a);
      }
    }
    return merged;
  }

  function handleClick(artifact: Artifact) {
    openArtifactTab(artifact);
    activeNavItem.set('chat');
  }

  function iconForType(type: string): string {
    switch (type) {
      case 'file': return '\uD83D\uDCC4';
      case 'command_output': return '\uD83D\uDCBB';
      default: return '\uD83D\uDCC1';
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
</script>

<div class="browser">
  <div class="browser-header">
    <h2 class="browser-title">Artifacts</h2>
    <span class="browser-count">{allArtifacts.length} files</span>
  </div>

  {#if loading && allArtifacts.length === 0}
    <div class="empty-state">Loading artifacts...</div>
  {:else if allArtifacts.length === 0}
    <div class="empty-state">No artifacts generated yet</div>
  {:else}
    <div class="artifact-grid">
      {#each allArtifacts as artifact (artifact.id)}
        <button class="artifact-card" on:click={() => handleClick(artifact)}>
          <div class="artifact-icon">{iconForType(artifact.type)}</div>
          <div class="artifact-name">{artifact.title}</div>
          <div class="artifact-meta">
            {formatSize(artifact.size_bytes)} &middot; {artifact.language || artifact.type}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .browser {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .browser-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 20px;
  }

  .browser-title {
    font-family: 'Exo 2', sans-serif;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .browser-count {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: var(--text-tertiary);
  }

  .artifact-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 10px;
    flex: 1;
    align-content: start;
    overflow-y: auto;
  }

  .artifact-card {
    padding: 16px;
    border-radius: var(--radius);
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
    cursor: pointer;
    transition: all 200ms ease;
    text-align: left;
    font-family: inherit;
    color: inherit;
  }

  .artifact-card:hover {
    border-color: var(--accent-border-active);
    box-shadow: var(--accent-glow);
    transform: translateY(-1px);
  }

  .artifact-icon { font-size: 20px; margin-bottom: 8px; }

  .artifact-name {
    font-size: 13px; font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .artifact-meta {
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }

  .empty-state {
    color: var(--text-tertiary);
    font-size: 14px;
    text-align: center;
    padding: 60px 0;
  }
</style>
