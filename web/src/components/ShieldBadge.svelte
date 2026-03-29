<script lang="ts">
  import { onMount } from 'svelte';
  import { ShieldCheck } from 'lucide-svelte';
  import { getStatus } from '../lib/api';

  let status = 'Active';
  let budgetPercent = 0;
  let tier2Calls = '0/50';

  onMount(async () => {
    try {
      const s = await getStatus();
      status = 'Active';
    } catch {
      status = 'Unavailable';
    }
  });
</script>

<div class="shield-status">
  <ShieldCheck size={14} />
  <span>Shield: <strong class:active={status === 'Active'} class:unavailable={status !== 'Active'}>{status}</strong></span>
</div>
<div class="shield-bar">
  <div class="shield-bar-fill" style="width: {budgetPercent}%"></div>
</div>
<div class="shield-detail">Tier 2: {tier2Calls} calls today</div>

<style>
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
  }
</style>
