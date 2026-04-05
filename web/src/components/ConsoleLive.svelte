<script lang="ts">
  import { onMount, afterUpdate, tick } from 'svelte';
  import { getLogs } from '../lib/api';
  import { logEntries, setLogEntries, consoleLive } from '../stores/console';
  import type { LogEntry } from '../stores/console';

  type LevelFilter = '' | 'debug' | 'info' | 'warn' | 'error';
  let activeLevelFilter: LevelFilter = '';
  const levelOptions: { id: LevelFilter; label: string }[] = [
    { id: '', label: 'All' },
    { id: 'debug', label: 'Debug' },
    { id: 'info', label: 'Info' },
    { id: 'warn', label: 'Warn' },
    { id: 'error', label: 'Error' },
  ];
  let searchQuery = '';
  let debounceTimer: ReturnType<typeof setTimeout>;

  let logEl: HTMLDivElement;
  let autoScroll = true;

  let logHasMore = false;
  let logLoadedCount = 0;
  let logLoading = false;

  let expandedEntries: Set<number> = new Set();

  onMount(async () => {
    try {
      const data = await getLogs(200);
      if (data.entries && data.entries.length > 0) {
        setLogEntries(data.entries);
        logLoadedCount = data.entries.length;
        logHasMore = data.has_more;
      }
    } catch {
      /* engine may not be ready */
    }
  });

  afterUpdate(() => {
    if (autoScroll && $consoleLive && logEl) {
      logEl.scrollTop = logEl.scrollHeight;
    }
  });

  function matchesLevel(entry: LogEntry, level: LevelFilter): boolean {
    if (!level) return true;
    return entry.level === level;
  }

  function matchesSearch(entry: LogEntry): boolean {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    if ((entry.event || '').toLowerCase().includes(q)) return true;
    if (entry.data) {
      const dataStr = JSON.stringify(entry.data).toLowerCase();
      if (dataStr.includes(q)) return true;
    }
    return false;
  }

  $: filteredEntries = $logEntries.filter(e => matchesLevel(e, activeLevelFilter) && matchesSearch(e));

  function handleSearchInput() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => { filteredEntries = filteredEntries; }, 300);
  }

  function handleScroll() {
    if (!logEl) return;
    const dist = logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight;
    autoScroll = dist < 50;
    if (logEl.scrollTop < 60 && logHasMore && !logLoading) {
      loadOlderLogs();
    }
  }

  function scrollToBottom() {
    if (logEl) logEl.scrollTo({ top: logEl.scrollHeight, behavior: 'smooth' });
    autoScroll = true;
  }

  function toggleExpand(idx: number) {
    if (expandedEntries.has(idx)) {
      expandedEntries.delete(idx);
    } else {
      expandedEntries.add(idx);
    }
    expandedEntries = expandedEntries;
  }

  async function loadOlderLogs() {
    if (logLoading || !logHasMore) return;
    logLoading = true;
    try {
      const data = await getLogs(200, '', '', logLoadedCount);
      if (data.entries && data.entries.length > 0) {
        const oldScroll = logEl ? logEl.scrollHeight : 0;
        setLogEntries([...data.entries, ...$logEntries]);
        logLoadedCount += data.entries.length;
        logHasMore = data.has_more;
        await tick();
        if (logEl) {
          logEl.scrollTop = logEl.scrollHeight - oldScroll;
        }
      } else {
        logHasMore = false;
      }
    } catch {
      /* ignore */
    }
    logLoading = false;
  }

  function formatTime(ts: string): string {
    if (!ts) return '';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return ts.slice(0, 23) || ts;
    const ms = String(d.getMilliseconds()).padStart(3, '0');
    const base = d.toLocaleString([], { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
    return `${base}.${ms}`;
  }

  function buildSummary(entry: LogEntry): string {
    const d = entry.data || {};
    const evt = entry.event || '';

    if (evt.includes('shield')) {
      const tool = d.tool || d.tool_name || '';
      const decision = d.decision || '';
      const tier = d.tier !== undefined ? `Tier ${d.tier}` : '';
      const ms = d.duration_ms !== undefined ? `${d.duration_ms}ms` : '';
      return [tool, decision, tier, ms].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('executor') || evt.includes('tool_call')) {
      const tool = d.tool || d.tool_name || '';
      const success = d.success !== undefined ? (d.success ? 'completed' : 'failed') : '';
      const ms = d.ms !== undefined ? `${d.ms}ms` : (d.duration_ms !== undefined ? `${d.duration_ms}ms` : '');
      const err = d.error || '';
      return [tool, success, ms, err].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('llm')) {
      const provider = d.provider || '';
      const model = d.model || '';
      const input = d.input_tokens !== undefined ? `${d.input_tokens}\u2192${d.output_tokens || 0} tokens` : '';
      const ms = d.duration_ms !== undefined ? `${(d.duration_ms / 1000).toFixed(1)}s` : '';
      return [provider ? `${provider}/${model}` : model, input, ms].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('memory')) {
      const op = d.operation || '';
      const query = d.query ? `"${d.query}"` : '';
      const ms = d.duration_ms !== undefined ? `${d.duration_ms}ms` : '';
      return [op, query, ms].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('mcp')) {
      const server = d.server || '';
      const mEvt = d.event || '';
      return [server, mEvt].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('session')) {
      const sEvt = d.event || '';
      const mode = d.mode || '';
      return [sEvt, mode].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('compaction')) {
      const before = d.before_tokens || 0;
      const after = d.after_tokens || 0;
      return before ? `${before}\u2192${after} tokens` : '';
    }
    if (evt.includes('generation')) {
      const type = d.type || '';
      const provider = d.provider || '';
      const ms = d.duration_ms !== undefined ? `${d.duration_ms}ms` : '';
      const bytes = d.output_bytes ? `${(d.output_bytes / 1024).toFixed(0)}KB` : '';
      return [type, provider, ms, bytes].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('tier3')) {
      const status = d.status || '';
      const tool = d.tool || '';
      const ms = d.response_time_ms ? `${d.response_time_ms}ms` : '';
      return [status, tool, ms].filter(Boolean).join(' \u00B7 ');
    }
    if (evt.includes('sandbox')) {
      const mode = d.mode || '';
      const ver = d.version ? `V${d.version}` : '';
      const fs = d.filesystem ? 'filesystem' : '';
      const net = d.network ? 'network' : '';
      const reason = d.reason || '';
      const caps = [fs, net].filter(Boolean).join(' + ');
      return [mode, ver, caps, reason].filter(Boolean).join(' \u00B7 ');
    }

    const vals = Object.values(d).slice(0, 3).map(v => String(v)).join(' \u00B7 ');
    return vals;
  }
</script>

<div class="console-header">
  <div class="filter-bar">
    {#each levelOptions as opt}
      <button
        class="filter-btn"
        class:active={activeLevelFilter === opt.id}
        class:warn={opt.id === 'warn' && activeLevelFilter === 'warn'}
        class:error={opt.id === 'error' && activeLevelFilter === 'error'}
        on:click={() => { activeLevelFilter = opt.id; }}
      >{opt.label}</button>
    {/each}
  </div>
  <div class="search-and-live">
    <input
      type="text"
      class="search-input"
      placeholder="Search..."
      bind:value={searchQuery}
      on:input={handleSearchInput}
    />
    <button class="live-toggle" class:paused={!$consoleLive} on:click={() => consoleLive.update(v => !v)}>
      <span class="live-dot" class:paused={!$consoleLive}></span>
      {$consoleLive ? 'Live' : 'Paused'}
    </button>
  </div>
</div>

<div class="log-entries" bind:this={logEl} on:scroll={handleScroll}>
  {#if logLoading}
    <div class="load-more">Loading older entries...</div>
  {:else if logHasMore}
    <div class="load-more subtle">Scroll up for more</div>
  {/if}
  {#if filteredEntries.length === 0}
    <div class="empty-state">No log entries yet</div>
  {:else}
    {#each filteredEntries as entry, i (i)}
      <button class="log-entry" class:warn={entry.level === 'warn'} class:error={entry.level === 'error'} on:click={() => toggleExpand(i)}>
        <span class="entry-time">{formatTime(entry.timestamp)}</span>
        <span class="entry-level" class:info={entry.level === 'info'} class:warn={entry.level === 'warn'} class:error={entry.level === 'error'}>{entry.level}</span>
        <span class="entry-event">{entry.event}</span>
        <span class="entry-summary">{buildSummary(entry)}</span>
      </button>
      {#if expandedEntries.has(i)}
        <pre class="entry-detail">{JSON.stringify(entry.data || {}, null, 2)}</pre>
      {/if}
    {/each}
  {/if}
</div>

{#if !autoScroll}
  <button class="jump-bottom" on:click={scrollToBottom}>&darr; Jump to bottom</button>
{/if}

<style>
  .console-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    flex-shrink: 0;
    flex-wrap: wrap;
    margin-bottom: 8px;
  }

  .filter-bar {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }

  .filter-btn {
    padding: 4px 10px;
    border-radius: 12px;
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
  .filter-btn.warn { background: rgba(255, 171, 0, 0.15); color: var(--warning); border-color: var(--warning); }
  .filter-btn.error { background: rgba(255, 80, 80, 0.15); color: var(--error); border-color: var(--error); }

  .search-and-live {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .search-input {
    width: 160px;
    padding: 4px 10px;
    border: 1px solid var(--accent-border);
    border-radius: 12px;
    background: none;
    color: var(--text-primary);
    font-size: 12px;
    font-family: inherit;
    outline: none;
  }
  .search-input::placeholder { color: var(--text-tertiary); }
  .search-input:focus { border-color: var(--accent-border-active); }

  .live-toggle {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 4px 10px;
    border: none;
    background: none;
    color: var(--text-secondary);
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    cursor: pointer;
  }

  .live-dot {
    width: 6px; height: 6px;
    border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 6px rgba(0, 230, 118, 0.4);
  }
  .live-dot.paused {
    background: var(--text-tertiary);
    box-shadow: none;
  }

  .log-entries {
    flex: 1;
    overflow-y: auto;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    line-height: 1.5;
  }

  .log-entry {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 8px;
    width: 100%;
    border: none;
    background: none;
    border-bottom: 1px solid var(--accent-border);
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    text-align: left;
    cursor: pointer;
    transition: background 100ms ease;
  }
  .log-entry:hover { background: var(--accent-ghost); }
  .log-entry.warn { color: var(--warning); border-left: 2px solid rgba(255, 171, 0, 0.3); }
  .log-entry.error { color: var(--error); border-left: 2px solid rgba(255, 61, 90, 0.3); background: rgba(255, 61, 90, 0.02); }

  .entry-time { color: var(--text-tertiary); font-size: 11px; flex-shrink: 0; white-space: nowrap; }

  .entry-level {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    padding: 1px 5px;
    border-radius: 3px;
    flex-shrink: 0;
    width: 40px;
    text-align: center;
  }
  .entry-level.info { color: var(--text-tertiary); }
  .entry-level.warn { color: var(--warning); background: rgba(255, 171, 0, 0.1); }
  .entry-level.error { color: var(--error); background: rgba(255, 61, 90, 0.1); }

  .entry-event { color: var(--accent-dim); flex-shrink: 0; white-space: nowrap; }

  .entry-summary { color: var(--text-secondary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }

  .entry-detail {
    padding: 8px 12px;
    margin: 0 0 4px 100px;
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
    border-radius: var(--radius);
    font-size: 11px;
    color: var(--text-secondary);
    line-height: 1.4;
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .jump-bottom {
    position: absolute;
    bottom: 50px;
    left: 50%;
    transform: translateX(-50%);
    padding: 4px 14px;
    border-radius: 12px;
    border: 1px solid var(--accent-border-active);
    background: rgba(12, 16, 28, 0.92);
    backdrop-filter: blur(12px);
    color: var(--accent);
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    cursor: pointer;
    z-index: 5;
  }
  .jump-bottom:hover { background: var(--accent-subtle); }

  .empty-state {
    color: var(--text-tertiary);
    font-size: 14px;
    text-align: center;
    padding: 60px 0;
    font-family: 'Exo 2', sans-serif;
  }

  .load-more {
    text-align: center;
    padding: 6px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--accent-dim);
    flex-shrink: 0;
  }
  .load-more.subtle {
    color: var(--text-tertiary);
    opacity: 0.5;
  }
</style>
