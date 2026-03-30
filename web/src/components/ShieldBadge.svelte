<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ShieldCheck, ShieldX, Gauge } from 'lucide-svelte';
  import { getStatus } from '../lib/api';

  let active = false;
  let tier2Used = 0;
  let tier2Budget = 50;
  let pollTimer: ReturnType<typeof setInterval>;

  $: budgetPercent = tier2Budget > 0 ? Math.min(100, (tier2Used / tier2Budget) * 100) : 0;
  $: barColor = budgetPercent >= 100 ? 'var(--error)' : budgetPercent >= 80 ? 'var(--warning)' : '';
  $: shieldColor = active ? 'var(--success)' : 'var(--error)';
  $: gaugeColor = budgetPercent >= 100 ? 'var(--error)' : budgetPercent >= 80 ? 'var(--warning)' : 'var(--accent)';

  async function refresh() {
    try {
      const s = await getStatus();
      if (s.shield) {
        active = s.shield.active;
        tier2Used = s.shield.tier2_used;
        tier2Budget = s.shield.tier2_budget;
      } else {
        active = true;
      }
    } catch {
      active = false;
    }
  }

  onMount(() => {
    refresh();
    pollTimer = setInterval(refresh, 15000);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });
</script>

<div class="shield-badge">
  <div class="badge-full">
    <div class="shield-status">
      {#if active}
        <ShieldCheck size={14} />
        <span>Shield: <strong class="active">Active</strong></span>
      {:else}
        <ShieldX size={14} />
        <span>Shield: <strong class="unavailable">Down</strong></span>
      {/if}
    </div>
    <div class="shield-bar">
      <div class="shield-bar-fill" style="width: {budgetPercent}%; {barColor ? `background: ${barColor}` : ''}"></div>
    </div>
    <div class="shield-detail">Tier 2: {tier2Used}/{tier2Budget} calls today</div>
  </div>

  <div class="badge-icons" title="Shield: {active ? 'Active' : 'Down'} · Tier 2: {tier2Used}/{tier2Budget}">
    <div class="icon-row">
      {#if active}
        <ShieldCheck size={16} style="color: {shieldColor}" />
      {:else}
        <ShieldX size={16} style="color: {shieldColor}" />
      {/if}
      <Gauge size={14} style="color: {gaugeColor}" />
    </div>
  </div>
</div>

<style>
  .badge-icons {
    display: none;
  }

  .shield-status {
    display: flex; align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 8px;
  }

  strong.active { color: var(--success); }
  strong.unavailable { color: var(--error); }

  .shield-bar {
    height: 3px;
    background: var(--accent-subtle);
    border-radius: 2px;
    overflow: hidden;
    margin-bottom: 6px;
  }

  .shield-bar-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-dim));
    border-radius: 2px;
    transition: width 500ms ease;
  }

  .shield-detail {
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }

  .icon-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 4px 0;
  }

  @media (max-width: 1200px) {
    .badge-full { display: none; }
    .badge-icons { display: block; }
  }

  @media (max-width: 800px) {
    .badge-full { display: block; }
    .badge-icons { display: none; }
  }
</style>
