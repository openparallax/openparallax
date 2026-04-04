<script lang="ts">
  import { onMount, afterUpdate, tick } from 'svelte';
  import { getLogs, getAudit, getMetrics, getDailyTokens } from '../lib/api';
  import { logEntries, setLogEntries, consoleLive } from '../stores/console';
  import type { LogEntry } from '../stores/console';

  type Tab = 'metrics' | 'live' | 'audit';
  let activeTab: Tab = 'metrics';

  type Filter = 'all' | 'shield' | 'tools' | 'llm' | 'memory' | 'errors';
  let activeFilterList: Filter[] = ['all'];
  const filterOptions: { id: Filter; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'shield', label: 'Shield' },
    { id: 'tools', label: 'Tools' },
    { id: 'llm', label: 'LLM' },
    { id: 'memory', label: 'Memory' },
    { id: 'errors', label: 'Errors' },
  ];
  let searchQuery = '';
  let debounceTimer: ReturnType<typeof setTimeout>;

  let logEl: HTMLDivElement;
  let autoScroll = true;

  let auditEntries: any[] = [];
  let auditChainValid = true;
  let auditChainBreakAt = -1;
  let auditTotal = 0;
  let expandedEntries: Set<number> = new Set();

  let auditEl: HTMLDivElement;
  let auditAutoScroll = false;

  type AuditFilter = 'all' | 'executed' | 'blocked' | 'failed' | 'sessions';
  let activeAuditFilter: AuditFilter = 'all';
  const auditFilterOptions: { id: AuditFilter; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'executed', label: 'Executed' },
    { id: 'blocked', label: 'Blocked' },
    { id: 'failed', label: 'Failed' },
    { id: 'sessions', label: 'Sessions' },
  ];

  let metricsSummary: any = null;
  let dailyTokenData: any[] = [];
  let metricsPeriod: string = 'daily';
  let metricsLoading = false;

  onMount(async () => {
    try {
      const data = await getLogs(200);
      if (data.entries && data.entries.length > 0) {
        setLogEntries(data.entries);
      }
    } catch {
      /* engine may not be ready */
    }
    loadMetrics();
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
    if (filters.includes('shield') && (evt.includes('shield') || evt.includes('ifc') || evt.includes('protection') || evt.includes('otr'))) return true;
    if (filters.includes('tools') && (evt.includes('tool') || evt.includes('executor') || evt.includes('mcp'))) return true;
    if (filters.includes('llm') && (evt.includes('llm') || evt.includes('compaction') || evt.includes('message_complete') || evt.includes('response_complete'))) return true;
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

  function handleAuditScroll() {
    if (!auditEl) return;
    const dist = auditEl.scrollHeight - auditEl.scrollTop - auditEl.clientHeight;
    auditAutoScroll = dist >= 50;
  }

  function scrollAuditToBottom() {
    if (auditEl) auditEl.scrollTo({ top: auditEl.scrollHeight, behavior: 'smooth' });
    auditAutoScroll = false;
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
    await tick();
    if (auditEl) {
      auditEl.scrollTop = auditEl.scrollHeight;
    }
  }

  async function loadMetrics() {
    metricsLoading = true;
    try {
      const [summary, daily] = await Promise.all([
        getMetrics(metricsPeriod),
        getDailyTokens(metricsPeriod === 'daily' ? 7 : metricsPeriod === 'weekly' ? 30 : metricsPeriod === 'monthly' ? 90 : 365),
      ]);
      metricsSummary = summary;
      dailyTokenData = daily || [];
    } catch {
      metricsSummary = null;
      dailyTokenData = [];
    }
    metricsLoading = false;
  }

  function setMetricsPeriod(period: string) {
    metricsPeriod = period;
    loadMetrics();
  }

  function switchTab(tab: Tab) {
    activeTab = tab;
    if (tab === 'audit' && auditEntries.length === 0) {
      loadAudit();
    }
    if (tab === 'metrics' && !metricsSummary) {
      loadMetrics();
    }
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

  function auditEventLabel(eventType: number): string {
    switch (eventType) {
      case 1: return 'PROPOSED';
      case 2: return 'EVALUATED';
      case 3: return 'APPROVED';
      case 4: return 'BLOCKED';
      case 5: return 'EXECUTED';
      case 6: return 'FAILED';
      case 7: return 'SHIELD_ERROR';
      case 8: return 'CANARY_OK';
      case 9: return 'CANARY_MISSING';
      case 10: return 'RATE_LIMIT';
      case 11: return 'BUDGET_OUT';
      case 12: return 'SELF_PROTECT';
      case 13: return 'TXN_BEGIN';
      case 14: return 'TXN_COMMIT';
      case 15: return 'TXN_ROLLBACK';
      case 16: return 'INTEGRITY';
      case 17: return 'SESSION_START';
      case 18: return 'SESSION_END';
      default: return `EVENT_${eventType}`;
    }
  }

  interface AuditTriplet {
    action: string;
    session: string;
    timestamp: number;
    entries: any[];
    outcome: 'executed' | 'blocked' | 'failed' | 'pending';
  }

  $: auditTriplets = groupAuditEntries(auditEntries);

  $: filteredAuditTriplets = auditTriplets.filter(t => {
    if (activeAuditFilter === 'all') return true;
    if (activeAuditFilter === 'sessions') {
      return t.entries.some((e: any) => e.event_type === 17 || e.event_type === 18);
    }
    return t.outcome === activeAuditFilter;
  });

  function groupAuditEntries(entries: any[]): AuditTriplet[] {
    const groups: AuditTriplet[] = [];
    let current: AuditTriplet | null = null;

    for (const entry of entries) {
      const et = entry.event_type as number;
      if (et === 1) {
        if (current) groups.push(current);
        current = {
          action: entry.action_type || '',
          session: entry.session_id || '',
          timestamp: entry.timestamp,
          entries: [entry],
          outcome: 'pending',
        };
      } else if (current && (et === 2 || et === 3)) {
        current.entries.push(entry);
      } else if (current && et === 5) {
        current.entries.push(entry);
        current.outcome = 'executed';
        groups.push(current);
        current = null;
      } else if (current && et === 4) {
        current.entries.push(entry);
        current.outcome = 'blocked';
        groups.push(current);
        current = null;
      } else if (current && et === 6) {
        current.entries.push(entry);
        current.outcome = 'failed';
        groups.push(current);
        current = null;
      } else {
        if (current) groups.push(current);
        current = null;
        groups.push({
          action: entry.action_type || auditEventLabel(et),
          session: entry.session_id || '',
          timestamp: entry.timestamp,
          entries: [entry],
          outcome: et === 4 ? 'blocked' : et === 6 ? 'failed' : 'executed',
        });
      }
    }
    if (current) groups.push(current);
    return groups;
  }

  let expandedTriplets: Set<number> = new Set();

  function toggleTriplet(idx: number) {
    if (expandedTriplets.has(idx)) {
      expandedTriplets.delete(idx);
    } else {
      expandedTriplets.add(idx);
    }
    expandedTriplets = new Set(expandedTriplets);
  }

  function formatNumber(n: number): string {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
    return String(n);
  }

  function formatPct(n: number): string {
    return n.toFixed(1) + '%';
  }

  $: chartMax = dailyTokenData.length > 0
    ? Math.max(...dailyTokenData.map((d: any) => (d.input_tokens || 0) + (d.output_tokens || 0)), 1)
    : 1;

  function chartBarHeight(tokens: number): string {
    return Math.max((tokens / chartMax) * 100, 1) + '%';
  }

  function formatChartDate(dateStr: string): string {
    if (!dateStr) return '';
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
</script>

<div class="console">
  {#if activeTab === 'metrics'}
    <div class="console-header">
      <div class="filter-bar">
        {#each [['daily','Today'],['weekly','Week'],['monthly','Month'],['yearly','Year']] as [id, label]}
          <button
            class="filter-btn"
            class:active={metricsPeriod === id}
            on:click={() => setMetricsPeriod(id)}
          >{label}</button>
        {/each}
      </div>
      <button class="filter-btn" on:click={() => loadMetrics()}>Refresh</button>
    </div>

    <div class="metrics-content">
      {#if metricsLoading}
        <div class="empty-state">Loading metrics...</div>
      {:else if !metricsSummary}
        <div class="empty-state">No metrics data available</div>
      {:else}
        <div class="metrics-grid">
          <div class="metric-card">
            <div class="metric-label">Token Usage</div>
            <div class="metric-value">{formatNumber(metricsSummary.total_tokens || 0)}</div>
            <div class="metric-sub">
              <span class="metric-input">{formatNumber(metricsSummary.input_tokens || 0)} in</span>
              <span class="metric-sep">/</span>
              <span class="metric-output">{formatNumber(metricsSummary.output_tokens || 0)} out</span>
            </div>
          </div>
          <div class="metric-card">
            <div class="metric-label">LLM Calls</div>
            <div class="metric-value">{formatNumber(metricsSummary.llm_calls || 0)}</div>
            <div class="metric-sub">requests</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Messages</div>
            <div class="metric-value">{formatNumber(metricsSummary.messages || 0)}</div>
            <div class="metric-sub">exchanges</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Sessions</div>
            <div class="metric-value">{formatNumber(metricsSummary.sessions || 0)}</div>
            <div class="metric-sub">active</div>
          </div>
        </div>

        <div class="metrics-grid wide">
          <div class="metric-card wide">
            <div class="metric-label">Shield</div>
            <div class="metric-row">
              <div class="metric-stat">
                <span class="stat-value allow">{metricsSummary.shield_allow || 0}</span>
                <span class="stat-label">allow</span>
              </div>
              <div class="metric-stat">
                <span class="stat-value block">{metricsSummary.shield_block || 0}</span>
                <span class="stat-label">block</span>
              </div>
              <div class="metric-stat">
                <span class="stat-value escalate">{metricsSummary.shield_escalate || 0}</span>
                <span class="stat-label">escalate</span>
              </div>
            </div>
            <div class="metric-tiers">
              <span class="tier-badge">T0: {metricsSummary.shield_t0 || 0}</span>
              <span class="tier-badge">T1: {metricsSummary.shield_t1 || 0}</span>
              <span class="tier-badge">T2: {metricsSummary.shield_t2 || 0}</span>
            </div>
          </div>
          <div class="metric-card wide">
            <div class="metric-label">Tools</div>
            <div class="metric-row">
              <div class="metric-stat">
                <span class="stat-value">{metricsSummary.tool_calls || 0}</span>
                <span class="stat-label">total</span>
              </div>
              <div class="metric-stat">
                <span class="stat-value allow">{metricsSummary.tool_success_rate != null ? formatPct(metricsSummary.tool_success_rate) : '---'}</span>
                <span class="stat-label">success</span>
              </div>
              <div class="metric-stat">
                <span class="stat-value block">{metricsSummary.tool_failed || 0}</span>
                <span class="stat-label">failed</span>
              </div>
            </div>
          </div>
        </div>

        {#if dailyTokenData.length > 0}
          <div class="chart-section">
            <div class="chart-label">Daily Tokens</div>
            <div class="chart-container">
              {#each dailyTokenData as day}
                <div class="chart-col" title="{formatChartDate(day.date)}: {formatNumber((day.input_tokens || 0) + (day.output_tokens || 0))} tokens">
                  <div class="chart-bar-stack" style="height: {chartBarHeight((day.input_tokens || 0) + (day.output_tokens || 0))}">
                    <div class="chart-bar output" style="height: {(day.output_tokens || 0) / Math.max((day.input_tokens || 0) + (day.output_tokens || 0), 1) * 100}%"></div>
                    <div class="chart-bar input" style="height: {(day.input_tokens || 0) / Math.max((day.input_tokens || 0) + (day.output_tokens || 0), 1) * 100}%"></div>
                  </div>
                  <span class="chart-date">{formatChartDate(day.date)}</span>
                </div>
              {/each}
            </div>
            <div class="chart-legend">
              <span class="legend-item"><span class="legend-swatch input"></span>Input</span>
              <span class="legend-item"><span class="legend-swatch output"></span>Output</span>
            </div>
          </div>
        {/if}
      {/if}
    </div>

  {:else if activeTab === 'live'}
    <div class="console-header">
      <div class="filter-bar">
        {#each filterOptions as opt}
          <button
            class="filter-btn"
            class:active={activeFilterList.includes(opt.id)}
            on:click={() => toggleFilter(opt.id)}
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
      <div class="filter-bar">
        {#each auditFilterOptions as opt}
          <button
            class="filter-btn"
            class:active={activeAuditFilter === opt.id}
            on:click={() => { activeAuditFilter = opt.id; }}
          >{opt.label}</button>
        {/each}
      </div>
      <div class="audit-status">
        {#if auditChainValid}
          <span class="chain-valid">&check; Chain valid &middot; {auditTotal} entries</span>
        {:else}
          <span class="chain-broken">&cross; Chain broken at entry #{auditChainBreakAt}</span>
        {/if}
      </div>
    </div>

    <div class="log-entries" bind:this={auditEl} on:scroll={handleAuditScroll}>
      {#if filteredAuditTriplets.length === 0}
        <div class="empty-state">No audit entries yet</div>
      {:else}
        {#each filteredAuditTriplets as triplet, i (i)}
          <button
            class="triplet-row"
            class:executed={triplet.outcome === 'executed'}
            class:blocked={triplet.outcome === 'blocked'}
            class:failed={triplet.outcome === 'failed'}
            on:click={() => toggleTriplet(i)}
          >
            <span class="entry-time">{formatTime(new Date(triplet.timestamp).toISOString())}</span>
            <span class="triplet-action">{triplet.action}</span>
            <span class="triplet-flow-inline">
              {#each triplet.entries as entry, j}
                {#if j > 0}<span class="flow-arrow">&rarr;</span>{/if}
                <span class="flow-step">{auditEventLabel(entry.event_type)}</span>
              {/each}
            </span>
            <span class="triplet-outcome" class:executed={triplet.outcome === 'executed'} class:blocked={triplet.outcome === 'blocked'} class:failed={triplet.outcome === 'failed'}>
              {triplet.outcome.toUpperCase()}
            </span>
          </button>
          {#if expandedTriplets.has(i)}
            <div class="triplet-detail">
              {#each triplet.entries as entry, j (j)}
                <button class="triplet-entry" on:click|stopPropagation={() => toggleExpand(20000 + i * 10 + j)}>
                  <div class="triplet-entry-header">
                    <span class="entry-time">{formatTime(new Date(entry.timestamp).toISOString())}</span>
                    <span class="audit-type">{auditEventLabel(entry.event_type)}</span>
                    <span class="entry-hash" title={entry.hash}><span class="hash-label">hash:</span> {(entry.hash || '').slice(0, 12)}</span>
                    {#if entry.previous_hash}
                      <span class="entry-hash" title={entry.previous_hash}><span class="hash-label">prev:</span> {entry.previous_hash.slice(0, 12)}</span>
                    {/if}
                  </div>
                  {#if expandedEntries.has(20000 + i * 10 + j)}
                    <pre class="entry-detail">{(() => { try { const e = {...entry}; if (e.details_json) { try { e.details = JSON.parse(e.details_json); delete e.details_json; } catch {} } return JSON.stringify(e, null, 2); } catch { return JSON.stringify(entry, null, 2); } })()}</pre>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        {/each}
      {/if}
    </div>

    {#if auditAutoScroll}
      <button class="jump-bottom" on:click={scrollAuditToBottom}>&darr; Jump to bottom</button>
    {/if}
  {/if}

  <div class="console-tabs">
    <button class="console-tab" class:active={activeTab === 'metrics'} on:click={() => switchTab('metrics')}>Metrics</button>
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

  .audit-status {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
  }
  .chain-valid { color: var(--success); }
  .chain-broken { color: var(--error); }

  .triplet-row {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 8px;
    border: none;
    border-bottom: 1px solid var(--accent-border);
    background: none;
    cursor: pointer;
    text-align: left;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    transition: background 100ms ease;
  }
  .triplet-row:hover { background: var(--accent-ghost); }
  .triplet-row.blocked { border-left: 2px solid var(--error); }
  .triplet-row.failed { border-left: 2px solid var(--warning); }
  .triplet-row.executed { border-left: 2px solid var(--success); }

  .triplet-action { color: var(--accent-dim); font-weight: 500; white-space: nowrap; }

  .triplet-flow-inline {
    display: flex;
    align-items: center;
    gap: 3px;
    font-size: 10px;
    flex: 1;
    overflow: hidden;
  }

  .triplet-outcome {
    font-size: 10px;
    font-weight: 600;
    padding: 1px 6px;
    border-radius: 3px;
    flex-shrink: 0;
  }
  .triplet-outcome.executed { color: var(--success); background: rgba(0, 230, 118, 0.1); }
  .triplet-outcome.blocked { color: var(--error); background: rgba(255, 61, 90, 0.1); }
  .triplet-outcome.failed { color: var(--warning); background: rgba(255, 171, 0, 0.1); }

  .flow-arrow { color: var(--text-tertiary); }

  .flow-step {
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--accent-ghost);
    color: var(--accent-dim);
    font-size: 10px;
    white-space: nowrap;
  }

  .triplet-detail {
    padding: 4px 8px 8px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    border-bottom: 1px solid var(--accent-border);
  }

  .triplet-entry {
    border: none;
    background: none;
    text-align: left;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 4px 8px;
    border-left: 2px solid var(--accent-border);
    width: 100%;
  }
  .triplet-entry:hover { background: var(--accent-ghost); }

  .triplet-entry-header {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 11px;
  }

  .audit-type {
    color: var(--accent-dim);
    background: var(--accent-ghost);
    padding: 1px 5px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
  }

  .entry-hash {
    color: var(--text-tertiary);
    font-size: 10px;
  }
  .hash-label { color: var(--text-tertiary); opacity: 0.6; }

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

  /* Metrics styles */
  .metrics-content {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
    margin-bottom: 8px;
  }
  .metrics-grid.wide {
    grid-template-columns: repeat(2, 1fr);
  }

  .metric-card {
    padding: 12px 14px;
    border: 1px solid var(--accent-border);
    border-radius: var(--radius, 8px);
    background: var(--accent-ghost);
    backdrop-filter: blur(8px);
  }

  .metric-label {
    font-family: 'Exo 2', sans-serif;
    font-size: 11px;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 6px;
  }

  .metric-value {
    font-family: 'JetBrains Mono', monospace;
    font-size: 22px;
    font-weight: 600;
    color: var(--accent);
    line-height: 1.2;
  }

  .metric-sub {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--text-tertiary);
    margin-top: 4px;
  }
  .metric-input { color: var(--accent-dim); }
  .metric-sep { color: var(--text-tertiary); margin: 0 2px; }
  .metric-output { color: var(--text-tertiary); }

  .metric-row {
    display: flex;
    gap: 16px;
    margin-top: 6px;
  }

  .metric-stat {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .stat-value {
    font-family: 'JetBrains Mono', monospace;
    font-size: 16px;
    font-weight: 600;
    color: var(--accent);
  }
  .stat-value.allow { color: var(--success); }
  .stat-value.block { color: var(--error); }
  .stat-value.escalate { color: var(--warning); }

  .stat-label {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--text-tertiary);
    text-transform: uppercase;
  }

  .metric-tiers {
    display: flex;
    gap: 6px;
    margin-top: 8px;
  }

  .tier-badge {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--accent-dim);
    background: var(--accent-ghost);
    padding: 2px 6px;
    border-radius: 3px;
    border: 1px solid var(--accent-border);
  }

  .chart-section {
    margin-top: 8px;
    padding: 12px 14px;
    border: 1px solid var(--accent-border);
    border-radius: var(--radius, 8px);
    background: var(--accent-ghost);
  }

  .chart-label {
    font-family: 'Exo 2', sans-serif;
    font-size: 11px;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 10px;
  }

  .chart-container {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    height: 100px;
    padding-bottom: 20px;
    position: relative;
  }

  .chart-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    height: 100%;
    min-width: 0;
    position: relative;
  }

  .chart-bar-stack {
    width: 100%;
    max-width: 24px;
    display: flex;
    flex-direction: column;
    border-radius: 2px 2px 0 0;
    overflow: hidden;
    margin-top: auto;
  }

  .chart-bar {
    width: 100%;
    min-height: 1px;
  }
  .chart-bar.input {
    background: var(--accent);
    opacity: 0.9;
  }
  .chart-bar.output {
    background: var(--accent-dim);
    opacity: 0.5;
  }

  .chart-date {
    position: absolute;
    bottom: -18px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 8px;
    color: var(--text-tertiary);
    white-space: nowrap;
    transform: rotate(-45deg);
    transform-origin: top left;
  }

  .chart-legend {
    display: flex;
    gap: 12px;
    margin-top: 8px;
    justify-content: flex-end;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 4px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--text-tertiary);
  }

  .legend-swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }
  .legend-swatch.input {
    background: var(--accent);
    opacity: 0.9;
  }
  .legend-swatch.output {
    background: var(--accent-dim);
    opacity: 0.5;
  }
</style>
