<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { getAudit } from '../lib/api';

  let auditEntries: any[] = [];
  let auditChainValid = true;
  let auditChainBreakAt = -1;
  let auditTotal = 0;
  let expandedEntries: Set<number> = new Set();

  let auditEl: HTMLDivElement;
  let auditAutoScroll = false;

  type AuditFilter = 'all' | 'executed' | 'blocked' | 'failed' | 'sessions';
  let activeAuditFilterList: AuditFilter[] = ['all'];
  const auditFilterOptions: { id: AuditFilter; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'executed', label: 'Executed' },
    { id: 'blocked', label: 'Blocked' },
    { id: 'failed', label: 'Failed' },
    { id: 'sessions', label: 'Sessions' },
  ];

  let auditHasMore = false;
  let auditLoadedCount = 0;
  let auditLoading = false;

  function toggleAuditFilter(f: AuditFilter) {
    if (f === 'all') {
      activeAuditFilterList = ['all'];
    } else {
      let next = activeAuditFilterList.filter(x => x !== 'all');
      if (next.includes(f)) {
        next = next.filter(x => x !== f);
      } else {
        next = [...next, f];
      }
      activeAuditFilterList = next.length === 0 ? ['all'] : next;
    }
  }

  async function loadAudit() {
    try {
      const data = await getAudit(200);
      auditEntries = data.entries || [];
      auditChainValid = data.chain_valid;
      auditChainBreakAt = data.chain_break_at ?? -1;
      auditTotal = data.total_entries;
      auditLoadedCount = auditEntries.length;
      auditHasMore = data.has_more;
    } catch {
      auditEntries = [];
    }
    await tick();
    if (auditEl) {
      auditEl.scrollTop = auditEl.scrollHeight;
    }
  }

  async function loadOlderAudit() {
    if (auditLoading || !auditHasMore) return;
    auditLoading = true;
    try {
      const data = await getAudit(200, auditLoadedCount);
      if (data.entries && data.entries.length > 0) {
        const oldScroll = auditEl ? auditEl.scrollHeight : 0;
        auditEntries = [...data.entries, ...auditEntries];
        auditLoadedCount += data.entries.length;
        auditHasMore = data.has_more;
        await tick();
        if (auditEl) {
          auditEl.scrollTop = auditEl.scrollHeight - oldScroll;
        }
      } else {
        auditHasMore = false;
      }
    } catch {
      /* ignore */
    }
    auditLoading = false;
  }

  function handleAuditScroll() {
    if (!auditEl) return;
    const dist = auditEl.scrollHeight - auditEl.scrollTop - auditEl.clientHeight;
    auditAutoScroll = dist >= 50;
    if (auditEl.scrollTop < 60 && auditHasMore && !auditLoading) {
      loadOlderAudit();
    }
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

  function formatTime(ts: string): string {
    if (!ts) return '';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return ts.slice(0, 23) || ts;
    const ms = String(d.getMilliseconds()).padStart(3, '0');
    const base = d.toLocaleString([], { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
    return `${base}.${ms}`;
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
    if (activeAuditFilterList.includes('all')) return true;
    if (activeAuditFilterList.includes('sessions') && t.entries.some((e: any) => e.event_type === 17 || e.event_type === 18)) return true;
    if (activeAuditFilterList.includes('executed') && t.outcome === 'executed') return true;
    if (activeAuditFilterList.includes('blocked') && t.outcome === 'blocked') return true;
    if (activeAuditFilterList.includes('failed') && t.outcome === 'failed') return true;
    return false;
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

  onMount(() => {
    loadAudit();
  });

  let expandedTriplets: Set<number> = new Set();

  function toggleTriplet(idx: number) {
    if (expandedTriplets.has(idx)) {
      expandedTriplets.delete(idx);
    } else {
      expandedTriplets.add(idx);
    }
    expandedTriplets = new Set(expandedTriplets);
  }
</script>

<div class="console-header">
  <div class="filter-bar">
    {#each auditFilterOptions as opt}
      <button
        class="filter-btn"
        class:active={activeAuditFilterList.includes(opt.id)}
        on:click={() => toggleAuditFilter(opt.id)}
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
  {#if auditLoading}
    <div class="load-more">Loading older entries...</div>
  {:else if auditHasMore}
    <div class="load-more subtle">Scroll up for more</div>
  {/if}
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

  .audit-status {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
  }
  .chain-valid { color: var(--success); }
  .chain-broken { color: var(--error); }

  .log-entries {
    flex: 1;
    overflow-y: auto;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    line-height: 1.5;
  }

  .entry-time { color: var(--text-tertiary); font-size: 11px; flex-shrink: 0; white-space: nowrap; }

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
