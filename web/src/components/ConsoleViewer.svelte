<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { getLogs, getAudit } from '../lib/api';
  import { logEntries, setLogEntries, consoleLive } from '../stores/console';
  import type { LogEntry } from '../stores/console';

  type Tab = 'live' | 'audit';
  let activeTab: Tab = 'live';

  type Filter = 'all' | 'shield' | 'tools' | 'llm' | 'memory' | 'errors';
  let activeFilterList: Filter[] = ['all'];
  let searchQuery = '';
  let debounceTimer: ReturnType<typeof setTimeout>;

  let logEl: HTMLDivElement;
  let autoScroll = true;

  let auditEntries: any[] = [];
  let auditChainValid = true;
  let auditChainBreakAt = -1;
  let auditTotal = 0;
  let expandedEntries: Set<number> = new Set();

  onMount(async () => {
    try {
      const data = await getLogs(200);
      if (data.entries && data.entries.length > 0) {
        setLogEntries(data.entries);
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

  function toggleFilter(f: Filter) {
    if (f === 'all') {
      activeFilterList = ['all'];
    } else {
      let next = activeFilterList.filter(x => x !== 'all');
      if (next.includes(f)) {
        next = next.filter(x => x !== f);
      } else {
        next = [...next, f];
      }
      activeFilterList = next.length === 0 ? ['all'] : next;
    }
  }

  function matchesFilter(entry: LogEntry, filters: Filter[]): boolean {
    if (filters.includes('all')) return true;
    const evt = entry.event || '';
    if (filters.includes('shield') && (evt.includes('shield') || evt.includes('ifc'))) return true;
    if (filters.includes('tools') && (evt.includes('tool') || evt.includes('executor'))) return true;
    if (filters.includes('llm') && (evt.includes('llm') || evt.includes('compaction'))) return true;
    if (filters.includes('memory') && evt.includes('memory')) return true;
    if (filters.includes('errors') && (entry.level === 'warn' || entry.level === 'error')) return true;
    return false;
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

  $: filteredEntries = $logEntries.filter(e => matchesFilter(e, activeFilterList) && matchesSearch(e));

  function handleSearchInput() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => { filteredEntries = filteredEntries; }, 300);
  }

  function handleScroll() {
    if (!logEl) return;
    const dist = logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight;
    autoScroll = dist < 50;
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

  async function loadAudit() {
    try {
      const data = await getAudit(200);
      auditEntries = data.entries || [];
      auditChainValid = data.chain_valid;
      auditChainBreakAt = data.chain_break_at ?? -1;
      auditTotal = data.total_entries;
    } catch {
      auditEntries = [];
    }
  }

  function switchTab(tab: Tab) {
    activeTab = tab;
    if (tab === 'audit' && auditEntries.length === 0) {
      loadAudit();
    }
  }

  function formatTime(ts: string): string {
    if (!ts) return '';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return ts.slice(11, 23) || ts;
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 });
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

    const vals = Object.values(d).slice(0, 3).map(v => String(v)).join(' \u00B7 ');
    return vals;
  }

  function auditEventLabel(eventType: number): string {
    switch (eventType) {
      case 1: return 'PROPOSED';
      case 2: return 'EVALUATED';
      case 3: return 'APPROVED';
      case 4: return 'BLOCKED';
      case 5: return 'EXECUTED';
      case 6: return 'FAILED';
      default: return `TYPE_${eventType}`;
    }
  }
</script>

<div class="console">
  {#if activeTab === 'live'}
    <div class="console-header">
      <div class="filter-bar">
        {#each [['all','All'],['shield','Shield'],['tools','Tools'],['llm','LLM'],['memory','Memory'],['errors','Errors']] as [id, label]}
          <button
            class="filter-btn"
            class:active={activeFilterList.includes(id)}
            on:click={() => toggleFilter(id)}
          >{label}</button>
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

  {:else}
    <div class="console-header">
      <div class="audit-status">
        {#if auditChainValid}
          <span class="chain-valid">&check; Chain valid &middot; {auditTotal} entries</span>
        {:else}
          <span class="chain-broken">&cross; Chain broken at entry #{auditChainBreakAt}</span>
        {/if}
      </div>
    </div>

    <div class="log-entries">
      {#if auditEntries.length === 0}
        <div class="empty-state">No audit entries yet</div>
      {:else}
        {#each auditEntries as entry, i (i)}
          <button class="log-entry audit-entry" on:click={() => toggleExpand(10000 + i)}>
            <span class="entry-time">{new Date(entry.timestamp).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>
            <span class="entry-level audit-type">{auditEventLabel(entry.event_type)}</span>
            <span class="entry-event">{entry.action_type || ''}</span>
            <span class="entry-hash" title={entry.hash}>{(entry.hash || '').slice(0, 8)}</span>
          </button>
          {#if expandedEntries.has(10000 + i)}
            <pre class="entry-detail">{JSON.stringify(entry, null, 2)}</pre>
          {/if}
        {/each}
      {/if}
    </div>
  {/if}

  <div class="console-tabs">
    <button class="console-tab" class:active={activeTab === 'live'} on:click={() => switchTab('live')}>Live Log</button>
    <button class="console-tab" class:active={activeTab === 'audit'} on:click={() => switchTab('audit')}>Audit Trail</button>
  </div>
</div>

<style>
  .console {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

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

  .entry-time { color: var(--text-tertiary); font-size: 11px; flex-shrink: 0; width: 90px; }

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

  .entry-event { color: var(--accent-dim); flex-shrink: 0; max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

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

  .audit-status {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
  }
  .chain-valid { color: var(--success); }
  .chain-broken { color: var(--error); }

  .audit-entry .entry-hash {
    color: var(--text-tertiary);
    font-size: 10px;
    flex-shrink: 0;
  }

  .audit-type {
    color: var(--accent-dim);
    background: var(--accent-ghost);
    width: 80px;
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

  .empty-state {
    color: var(--text-tertiary);
    font-size: 14px;
    text-align: center;
    padding: 60px 0;
    font-family: 'Exo 2', sans-serif;
  }
</style>
