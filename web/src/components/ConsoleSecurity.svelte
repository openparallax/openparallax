<script lang="ts">
  import { onMount } from 'svelte';
  import { getMetrics } from '../lib/api';

  let metrics: any = null;
  let period: string = 'daily';
  let loading = false;

  async function load() {
    loading = true;
    try {
      metrics = await getMetrics(period);
    } catch {
      metrics = null;
    }
    loading = false;
  }

  function setPeriod(p: string) {
    period = p;
    load();
  }

  function integrityAlert(v: number): boolean {
    return v > 0;
  }

  onMount(() => { load(); });
</script>

<div class="console-header">
  <div class="filter-bar">
    {#each [['daily','Today'],['weekly','Week'],['monthly','Month'],['yearly','Year']] as [id, label]}
      <button
        class="filter-pill"
        class:active={period === id}
        on:click={() => setPeriod(id)}
      >{label}</button>
    {/each}
  </div>
</div>

<div class="security-content">
{#if loading}
  <div class="metrics-loading">Loading security metrics...</div>
{:else if metrics}
  <!-- Security Integrity (the canary panel — should always be all-zero) -->
  <div class="metrics-section">
    <h3 class="section-title">Security Integrity</h3>
    <p class="section-subtitle">These counters should always be zero. Non-zero values indicate a security event.</p>
    <div class="metrics-grid">
      <div class="metric-card" class:alert={integrityAlert(metrics.security_integrity?.audit_chain_failures || 0)}>
        <div class="metric-value">{metrics.security_integrity?.audit_chain_failures || 0}</div>
        <div class="metric-label">Audit Chain Failures</div>
      </div>
      <div class="metric-card" class:alert={integrityAlert(metrics.security_integrity?.hash_verifier_failures || 0)}>
        <div class="metric-value">{metrics.security_integrity?.hash_verifier_failures || 0}</div>
        <div class="metric-label">Hash Verifier Failures</div>
      </div>
      <div class="metric-card" class:alert={integrityAlert(metrics.security_integrity?.canary_token_failures || 0)}>
        <div class="metric-value">{metrics.security_integrity?.canary_token_failures || 0}</div>
        <div class="metric-label">Canary Token Failures</div>
      </div>
      <div class="metric-card" class:alert={integrityAlert(metrics.security_integrity?.agent_auth_failures || 0)}>
        <div class="metric-value">{metrics.security_integrity?.agent_auth_failures || 0}</div>
        <div class="metric-label">Agent Auth Failures</div>
      </div>
      <div class="metric-card" class:alert={integrityAlert(metrics.security_integrity?.agent_unexpected_exits || 0)}>
        <div class="metric-value">{metrics.security_integrity?.agent_unexpected_exits || 0}</div>
        <div class="metric-label">Unexpected Agent Exits</div>
      </div>
      <div class="metric-card" class:alert={integrityAlert(metrics.security_integrity?.protection_bypass_attempts || 0)}>
        <div class="metric-value">{metrics.security_integrity?.protection_bypass_attempts || 0}</div>
        <div class="metric-label">Protection Bypass Attempts</div>
      </div>
    </div>
  </div>

  <!-- Shield Decisions -->
  <div class="metrics-section">
    <h3 class="section-title">Shield Decisions</h3>
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="metric-value accent">{metrics.shield_summary?.shield_allow || 0}</div>
        <div class="metric-label">Allowed</div>
      </div>
      <div class="metric-card">
        <div class="metric-value warn">{metrics.shield_summary?.shield_block || 0}</div>
        <div class="metric-label">Blocked</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.shield_summary?.shield_escalate || 0}</div>
        <div class="metric-label">Escalated</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.shield_summary?.rate_limit_hit || 0}</div>
        <div class="metric-label">Rate Limited</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.shield_summary?.budget_exhausted || 0}</div>
        <div class="metric-label">Budget Exhausted</div>
      </div>
    </div>
  </div>

  <!-- Defense Activity -->
  <div class="metrics-section">
    <h3 class="section-title">Defense Activity</h3>
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="metric-value">{metrics.security_defenses?.protection_blocks || 0}</div>
        <div class="metric-label">Protection Layer Blocks</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.security_defenses?.tier3_requests || 0}</div>
        <div class="metric-label">Tier 3 Requests</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.security_defenses?.subagent_concurrency_cap_hits || 0}</div>
        <div class="metric-label">Sub-Agent Cap Hits</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.security_defenses?.subagent_timeout_kills || 0}</div>
        <div class="metric-label">Sub-Agent Timeouts</div>
      </div>
    </div>
  </div>

  <!-- IFC -->
  <div class="metrics-section">
    <h3 class="section-title">Information Flow Control</h3>
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="metric-value warn">{metrics.ifc?.blocks_total || 0}</div>
        <div class="metric-label">IFC Blocks</div>
      </div>
      <div class="metric-card">
        <div class="metric-value">{metrics.ifc?.audit_would_block_total || 0}</div>
        <div class="metric-label">Audit Would-Block</div>
      </div>
    </div>
  </div>

  <!-- Tool Stats (from existing data) -->
  <div class="metrics-section">
    <h3 class="section-title">Tool Execution</h3>
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="metric-value">{metrics.daily_metrics?.tool_calls || 0}</div>
        <div class="metric-label">Total Calls</div>
      </div>
      <div class="metric-card">
        <div class="metric-value accent">{metrics.daily_metrics?.tool_success || 0}</div>
        <div class="metric-label">Successful</div>
      </div>
      <div class="metric-card">
        <div class="metric-value warn">{metrics.daily_metrics?.tool_failed || 0}</div>
        <div class="metric-label">Failed</div>
      </div>
    </div>
  </div>
{:else}
  <div class="metrics-empty">No security metrics available.</div>
{/if}
</div>

<style>
  .console-header {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--accent-border, rgba(0, 220, 255, 0.15));
  }
  .filter-bar {
    display: flex;
    gap: 0.5rem;
  }
  .filter-pill {
    padding: 0.25rem 0.75rem;
    border-radius: 999px;
    border: 1px solid var(--accent-border, rgba(0, 220, 255, 0.15));
    background: transparent;
    color: var(--accent-dim, rgba(0, 220, 255, 0.5));
    cursor: pointer;
    font-size: 0.75rem;
    font-family: 'Exo 2', sans-serif;
    transition: all 0.2s;
  }
  .filter-pill:hover { border-color: var(--accent-base, rgb(0, 220, 255)); }
  .filter-pill.active {
    background: var(--accent-ghost, rgba(0, 220, 255, 0.08));
    border-color: var(--accent-base, rgb(0, 220, 255));
    color: var(--accent-base, rgb(0, 220, 255));
  }

  .security-content {
    flex: 1;
    overflow-y: auto;
  }

  .metrics-loading, .metrics-empty {
    padding: 2rem;
    text-align: center;
    color: var(--accent-dim, rgba(0, 220, 255, 0.5));
    font-size: 0.85rem;
  }

  .metrics-section {
    padding: 1rem;
    border-bottom: 1px solid var(--accent-border, rgba(0, 220, 255, 0.08));
  }
  .section-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--accent-dim, rgba(0, 220, 255, 0.6));
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin: 0 0 0.25rem 0;
  }
  .section-subtitle {
    font-size: 0.7rem;
    color: var(--accent-subtle, rgba(0, 220, 255, 0.3));
    margin: 0 0 0.75rem 0;
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 0.5rem;
  }
  .metric-card {
    padding: 0.75rem;
    border-radius: 8px;
    border: 1px solid var(--accent-border, rgba(0, 220, 255, 0.1));
    background: var(--accent-ghost, rgba(0, 220, 255, 0.03));
    text-align: center;
  }
  .metric-card.alert {
    border-color: rgba(255, 80, 80, 0.6);
    background: rgba(255, 80, 80, 0.08);
  }
  .metric-value {
    font-size: 1.5rem;
    font-weight: 700;
    font-family: 'JetBrains Mono', monospace;
    color: var(--accent-dim, rgba(0, 220, 255, 0.7));
  }
  .metric-value.accent { color: var(--accent-base, rgb(0, 220, 255)); }
  .metric-value.warn { color: rgba(255, 180, 50, 0.9); }
  .metric-card.alert .metric-value { color: rgba(255, 80, 80, 0.95); }
  .metric-label {
    font-size: 0.65rem;
    color: var(--accent-subtle, rgba(0, 220, 255, 0.4));
    margin-top: 0.25rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }
</style>
