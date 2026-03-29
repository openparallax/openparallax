<script lang="ts">
  import { onMount } from 'svelte';
  import { Search } from 'lucide-svelte';
  import { searchMemory, readMemory } from '../lib/api';
  import { renderMarkdown } from '../lib/format';

  let query = '';
  let results: any[] = [];
  let searching = false;
  let memoryContent = '';
  let debounceTimer: ReturnType<typeof setTimeout>;

  onMount(async () => {
    try {
      const data = await readMemory('MEMORY');
      memoryContent = data.content;
    } catch {
      memoryContent = '';
    }
  });

  function handleInput() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(handleSearch, 300);
  }

  async function handleSearch() {
    if (!query.trim()) {
      results = [];
      searching = false;
      return;
    }
    searching = true;
    try {
      const res = await searchMemory(query);
      results = Array.isArray(res) ? res : [];
    } catch {
      results = [];
    } finally {
      searching = false;
    }
  }

  $: renderedMemory = memoryContent ? renderMarkdown(memoryContent) : '';
</script>

<div class="dashboard">
  <div class="dashboard-header">
    <h2 class="dashboard-title">Memory</h2>
  </div>

  <div class="search-bar">
    <Search size={14} />
    <input
      type="text"
      bind:value={query}
      on:input={handleInput}
      on:keydown={(e) => e.key === 'Enter' && handleSearch()}
      placeholder="Search memory..."
      class="search-input"
    />
  </div>

  {#if query.trim() && results.length > 0}
    <div class="search-results">
      {#each results as result, i (i)}
        <div class="result-card">
          <div class="result-path">{result.path || result.Path}</div>
          <div class="result-snippet">{result.snippet || result.Snippet}</div>
        </div>
      {/each}
    </div>
  {:else if query.trim() && !searching}
    <div class="empty-state">No results found</div>
  {:else if renderedMemory}
    <div class="memory-content markdown-content">
      {@html renderedMemory}
    </div>
  {:else}
    <div class="empty-state">No memory data available</div>
  {/if}
</div>

<style>
  .dashboard {
    width: 100%;
    max-width: 720px;
    align-self: flex-start;
  }

  .dashboard-header {
    margin-bottom: 16px;
  }

  .dashboard-title {
    font-family: 'Exo 2', sans-serif;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-radius: var(--radius);
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
    margin-bottom: 20px;
    color: var(--text-tertiary);
  }

  .search-input {
    flex: 1;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 14px;
    outline: none;
    font-family: inherit;
  }
  .search-input::placeholder { color: var(--text-tertiary); }

  .search-results {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .result-card {
    padding: 12px 14px;
    border-radius: var(--radius);
    background: var(--accent-ghost);
    border: 1px solid var(--accent-border);
  }

  .result-path {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: var(--accent-dim);
    margin-bottom: 4px;
  }

  .result-snippet {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .memory-content {
    padding: 16px;
    border-radius: var(--radius);
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
  }

  .empty-state {
    color: var(--text-tertiary);
    font-size: 14px;
    text-align: center;
    padding: 60px 0;
  }
</style>
