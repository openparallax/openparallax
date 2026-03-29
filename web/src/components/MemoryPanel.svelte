<script lang="ts">
  import { Search } from 'lucide-svelte';
  import { searchMemory } from '../lib/api';

  let query = '';
  let results: any[] = [];
  let searching = false;

  async function handleSearch() {
    if (!query.trim()) return;
    searching = true;
    try {
      results = await searchMemory(query) as any[];
    } catch {
      results = [];
    }
    searching = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') handleSearch();
  }
</script>

<div class="memory-panel">
  <div class="memory-search">
    <Search size={14} />
    <input
      type="text"
      bind:value={query}
      on:keydown={handleKeydown}
      placeholder="Search memory..."
      class="memory-search-input"
    />
  </div>

  {#if results.length > 0}
    <div class="memory-results">
      {#each results as result, i (i)}
        <div class="memory-result">
          <div class="memory-result-path">{result.path || result.Path}</div>
          <div class="memory-result-snippet">{result.snippet || result.Snippet}</div>
        </div>
      {/each}
    </div>
  {:else if query && !searching}
    <div class="empty-state">No results found</div>
  {:else}
    <div class="empty-state">Search your memory</div>
  {/if}
</div>

<style>
  .memory-panel { display: flex; flex-direction: column; gap: 12px; }

  .memory-search {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 12px;
    border-radius: var(--radius);
    background: rgba(12, 16, 28, 0.6);
    border: 1px solid var(--accent-border);
  }

  .memory-search-input {
    flex: 1; border: none; background: none;
    color: var(--text-primary);
    font-size: 13px; outline: none;
    font-family: inherit;
  }
  .memory-search-input::placeholder { color: var(--text-tertiary); }

  .memory-results { display: flex; flex-direction: column; gap: 8px; }

  .memory-result {
    padding: 10px;
    border-radius: var(--radius);
    background: var(--accent-ghost);
    border: 1px solid var(--accent-border);
  }

  .memory-result-path {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px; color: var(--accent-dim);
    margin-bottom: 4px;
  }

  .memory-result-snippet {
    font-size: 12px; color: var(--text-secondary);
    line-height: 1.5;
  }

  .empty-state {
    color: var(--text-tertiary);
    font-size: 13px;
    text-align: center;
    padding: 40px 0;
  }
</style>
