<script lang="ts">
  import { onMount } from 'svelte';
  import { getMetrics, getDailyTokens } from '../lib/api';

  let metricsSummary: any = null;
  let dailyTokenData: any[] = [];
  let metricsPeriod: string = 'daily';
  let metricsLoading = false;

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

  function formatNumber(n: number): string {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
    return String(n);
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

  onMount(() => {
    loadMetrics();
  });
</script>

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
    {@const perf = metricsSummary.performance || {}}
    {@const life = metricsSummary.lifetime || {}}
    {@const shield = metricsSummary.shield_summary || {}}
    {@const tools = metricsSummary.daily_metrics || {}}
    {@const tokens = metricsSummary.token_usage || {}}
    {@const topTools = metricsSummary.top_tools || []}
    {@const toolTotal = tools.tool_calls || 0}
    {@const toolSuccess = tools.tool_success || 0}
    {@const successRate = toolTotal > 0 ? Math.round((toolSuccess / toolTotal) * 100) : 0}
    <!-- Row 1: Usage -->
    <div class="metrics-grid">
      <div class="metric-card" title="Total input + output tokens consumed by LLM API calls in this period">
        <div class="metric-label">Token Usage</div>
        <div class="metric-value">{formatNumber(tokens.total || 0)}</div>
        <div class="metric-sub">
          <span class="metric-input">{formatNumber(tokens.input || 0)} in</span>
          <span class="metric-sep">/</span>
          <span class="metric-output">{formatNumber(tokens.output || 0)} out</span>
        </div>
      </div>
      <div class="metric-card" title="Number of LLM API requests. One message can trigger multiple calls due to tool-use rounds">
        <div class="metric-label">LLM Calls</div>
        <div class="metric-value">{formatNumber(tokens.llm_calls || 0)}</div>
        <div class="metric-sub">{perf.avg_latency_ms ? formatNumber(perf.avg_latency_ms) + 'ms avg' : 'requests'}</div>
      </div>
      <div class="metric-card" title="User + assistant messages in this period. Avg per session shown below">
        <div class="metric-label">Messages</div>
        <div class="metric-value">{formatNumber(metricsSummary.message_count || 0)}</div>
        <div class="metric-sub">{life.avg_msgs_per_session ? life.avg_msgs_per_session.toFixed(1) + '/session' : 'exchanges'}</div>
      </div>
      <div class="metric-card" title="Total sessions ever created (survives deletions). Period count shown below">
        <div class="metric-label">Sessions</div>
        <div class="metric-value">{formatNumber(life.total_sessions || 0)}</div>
        <div class="metric-sub">{metricsSummary.session_count || 0} this period</div>
      </div>
    </div>

    <!-- Row 2: Performance -->
    <div class="metrics-grid">
      <div class="metric-card" title="Latency percentiles for LLM API calls. P50 = median, P95/P99 = tail latency">
        <div class="metric-label">Latency</div>
        <div class="metric-value">{formatNumber(perf.p50_latency_ms || 0)}<span class="metric-unit">ms</span></div>
        <div class="metric-sub">
          p50 · p95: {formatNumber(perf.p95_latency_ms || 0)}ms · p99: {formatNumber(perf.p99_latency_ms || 0)}ms
        </div>
      </div>
      <div class="metric-card" title="Average tool-use rounds per LLM call. Higher = agent doing more work per message">
        <div class="metric-label">Rounds/Call</div>
        <div class="metric-value">{perf.avg_rounds_per_msg ? perf.avg_rounds_per_msg.toFixed(1) : '0'}</div>
        <div class="metric-sub">avg tool loops</div>
      </div>
      <div class="metric-card" title="Prompt cache hit rate. Higher = more tokens served from cache, lower cost">
        <div class="metric-label">Cache Rate</div>
        <div class="metric-value">{perf.cache_hit_rate ? (perf.cache_hit_rate * 100).toFixed(0) : '0'}<span class="metric-unit">%</span></div>
        <div class="metric-sub">{formatNumber(tokens.cache_read || 0)} cached tokens</div>
      </div>
      <div class="metric-card" title="Average tokens consumed per LLM API call. Monitors context size bloat">
        <div class="metric-label">Tokens/Call</div>
        <div class="metric-value">{formatNumber(perf.avg_tokens_per_msg || 0)}</div>
        <div class="metric-sub">avg per request</div>
      </div>
    </div>

    <!-- Row 3: Shield + Tools -->
    <div class="metrics-grid wide">
      <div class="metric-card wide" title="Security shield decisions. Every tool call is evaluated before execution">
        <div class="metric-label">Shield</div>
        <div class="metric-row">
          <div class="metric-stat">
            <span class="stat-value allow">{shield.shield_allow || 0}</span>
            <span class="stat-label">allow</span>
          </div>
          <div class="metric-stat">
            <span class="stat-value block">{shield.shield_block || 0}</span>
            <span class="stat-label">block</span>
          </div>
          <div class="metric-stat">
            <span class="stat-value escalate">{shield.shield_escalate || 0}</span>
            <span class="stat-label">escalate</span>
          </div>
        </div>
        <div class="metric-tiers">
          <span class="tier-badge">T0: {shield.shield_t0 || 0}</span>
          <span class="tier-badge">T1: {shield.shield_t1 || 0}</span>
          <span class="tier-badge">T2: {shield.shield_t2 || 0}</span>
        </div>
      </div>
      <div class="metric-card wide" title="Tool invocations: executed + blocked + load_tools. Success rate shown as gauge">
        <div class="metric-label">Tools</div>
        <div class="metric-row">
          <div class="metric-stat">
            <span class="stat-value">{toolTotal}</span>
            <span class="stat-label">total</span>
          </div>
          <div class="metric-stat">
            <span class="stat-value allow">{toolSuccess}</span>
            <span class="stat-label">success</span>
          </div>
          <div class="metric-stat">
            <span class="stat-value block">{tools.tool_failed || 0}</span>
            <span class="stat-label">failed</span>
          </div>
          {#if toolTotal > 0}
            <div class="gauge-wrap" title="{successRate}% success rate">
              <svg viewBox="0 0 36 20" class="gauge-svg">
                <path d="M3 18 A15 15 0 0 1 33 18" fill="none" stroke="var(--accent-ghost)" stroke-width="3" stroke-linecap="round"/>
                <path d="M3 18 A15 15 0 0 1 33 18" fill="none" stroke="var(--success)" stroke-width="3" stroke-linecap="round"
                  stroke-dasharray="{successRate * 0.47} 100"/>
              </svg>
              <span class="gauge-label">{successRate}%</span>
            </div>
          {/if}
        </div>
      </div>
    </div>

    <!-- Row 4: Top Tools -->
    {#if topTools.length > 0}
      {@const toolCounts = topTools.map(t => t.count || 0)}
      {@const maxToolCount = Math.max(...toolCounts, 1)}
      <div class="chart-section" title="Most frequently used tools in this period">
        <div class="chart-label">Top Tools</div>
        <div class="tool-bars">
          {#each topTools as tool}
            <div class="tool-bar-row">
              <span class="tool-bar-name">{tool.name}</span>
              <div class="tool-bar-track">
                <div class="tool-bar-fill" style="width: {(tool.count / maxToolCount) * 100}%"></div>
              </div>
              <span class="tool-bar-count">{tool.count}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Row 5: Daily Tokens Chart -->
    {#if dailyTokenData.length > 0}
      <div class="chart-section">
        <div class="chart-label">Daily Tokens</div>
        <div class="chart-container">
          {#each dailyTokenData as day}
            <div class="chart-col" title="{formatChartDate(day.date)}: {formatNumber((day.input_tokens || 0) + (day.output_tokens || 0))} tokens">
              <div class="chart-bar-stack" style="height: {chartBarHeight((day.input_tokens || 0) + (day.output_tokens || 0))}">
                <div class="chart-bar output" style="height: {Math.max((day.output_tokens || 0) / Math.max((day.input_tokens || 0) + (day.output_tokens || 0), 1) * 100, day.output_tokens ? 4 : 0)}%"></div>
                <div class="chart-bar input" style="height: {Math.max((day.input_tokens || 0) / Math.max((day.input_tokens || 0) + (day.output_tokens || 0), 1) * 100, day.input_tokens ? 4 : 0)}%"></div>
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

    <!-- Row 6: Lifetime -->
    <div class="metrics-grid lifetime" title="All-time totals independent of period filter">
      <div class="metric-card" title="Total sessions ever created, including deleted ones">
        <div class="metric-label">Lifetime Sessions</div>
        <div class="metric-value sub">{formatNumber(life.total_sessions || 0)}</div>
      </div>
      <div class="metric-card" title="Total messages across all sessions ever">
        <div class="metric-label">Lifetime Messages</div>
        <div class="metric-value sub">{formatNumber(life.total_messages || 0)}</div>
      </div>
      <div class="metric-card" title="Total tokens consumed across all LLM calls ever">
        <div class="metric-label">Lifetime Tokens</div>
        <div class="metric-value sub">{formatNumber(life.total_tokens || 0)}</div>
      </div>
      <div class="metric-card" title="Average number of messages per session across all sessions">
        <div class="metric-label">Avg Msgs/Session</div>
        <div class="metric-value sub">{life.avg_msgs_per_session ? life.avg_msgs_per_session.toFixed(1) : '0'}</div>
      </div>
    </div>
  {/if}
</div>

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

  .empty-state {
    color: var(--text-tertiary);
    font-size: 14px;
    text-align: center;
    padding: 60px 0;
    font-family: 'Exo 2', sans-serif;
  }

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
    background: #a78bfa;
    opacity: 0.85;
  }

  .chart-date {
    position: absolute;
    bottom: -16px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 9px;
    color: var(--text-tertiary);
    white-space: nowrap;
    text-align: center;
    left: 50%;
    transform: translateX(-50%);
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
    background: #a78bfa;
  }

  .metric-unit {
    font-size: 12px;
    font-weight: 400;
    color: var(--text-tertiary);
    margin-left: 2px;
  }

  .metric-value.sub {
    font-size: 18px;
  }

  .metrics-grid.lifetime {
    margin-top: 8px;
    opacity: 0.7;
  }

  .gauge-wrap {
    margin-left: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    flex-shrink: 0;
  }

  .gauge-svg {
    width: 40px;
    height: 22px;
  }

  .gauge-label {
    font-family: 'JetBrains Mono', monospace;
    font-size: 9px;
    color: var(--success);
    margin-top: -2px;
  }

  .tool-bars {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .tool-bar-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .tool-bar-name {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--text-tertiary);
    width: 100px;
    text-align: right;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .tool-bar-track {
    flex: 1;
    height: 6px;
    background: var(--accent-ghost);
    border-radius: 3px;
    overflow: hidden;
  }

  .tool-bar-fill {
    height: 100%;
    background: var(--accent-dim);
    border-radius: 3px;
    min-width: 2px;
    transition: width 300ms ease;
  }

  .tool-bar-count {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--accent-dim);
    width: 30px;
    text-align: right;
    flex-shrink: 0;
  }
</style>
