<script lang="ts">
  import { ChevronRight, ChevronDown } from 'lucide-svelte';
  import type { Thought } from '../lib/types';
  import type { PipelineStep } from '../stores/messages';

  // Finalized mode: server-side thoughts from a persisted message.
  export let thoughts: Thought[] = [];
  // Live mode: unified pipeline steps (reasoning + tool calls interleaved).
  export let steps: PipelineStep[] = [];
  export let live = false;

  let expanded = false;

  // Unified item type for rendering.
  interface DisplayItem {
    kind: 'reasoning' | 'tool_call';
    name: string;
    desc: string;
    blocked: boolean;
    failed: boolean;
  }

  $: items = buildItems(thoughts, steps);
  $: toolCount = items.filter(t => t.kind === 'tool_call').length;
  $: reasoningCount = items.filter(t => t.kind === 'reasoning').length;
  $: totalSteps = items.length;
  $: hasBlock = items.some(t => t.blocked);
  $: hasFail = items.some(t => t.failed);
  $: allComplete = !live && items.some(t => t.kind === 'tool_call');
  $: successCount = items.filter(t => t.kind === 'tool_call' && !t.blocked && !t.failed).length;

  function buildItems(th: Thought[], st: PipelineStep[]): DisplayItem[] {
    // Live mode: build from pipeline steps (already in chronological order).
    if (st.length > 0) {
      return st.map(s => {
        if (s.type === 'reasoning') {
          return { kind: 'reasoning', name: s.content || '', desc: '', blocked: false, failed: false };
        }
        const blocked = s.shieldVerdict?.decision === 'BLOCK';
        const failed = !blocked && s.result?.success === false;
        return {
          kind: 'tool_call',
          name: s.toolName || '',
          desc: s.result?.summary || s.summary || '',
          blocked,
          failed,
        };
      });
    }
    // Finalized mode: build from thoughts (already in order from server).
    if (th.length > 0) {
      return th.map(t => {
        if (t.stage === 'reasoning') {
          return { kind: 'reasoning', name: t.summary || '', desc: '', blocked: false, failed: false };
        }
        const blocked = t.detail?.shield === 'BLOCK';
        const failed = !blocked && t.detail?.success === false;
        const parsed = parseToolName(t);
        return { kind: 'tool_call', name: parsed.name, desc: parsed.desc, blocked, failed };
      });
    }
    return [];
  }

  function parseToolName(thought: Thought): { name: string; desc: string } {
    if (thought.detail?.tool_name) {
      const name = thought.detail.tool_name as string;
      const desc = thought.detail?.result_summary as string || thought.summary || '';
      return { name, desc };
    }
    const summary = thought.summary || '';
    for (const sep of [' \u2014 ', ' — ', ' \u2192 ']) {
      if (summary.includes(sep)) {
        const idx = summary.indexOf(sep);
        return { name: summary.slice(0, idx).trim(), desc: summary.slice(idx + sep.length).trim() };
      }
    }
    const parenIdx = summary.indexOf('(');
    if (parenIdx > 0) {
      return { name: summary.slice(0, parenIdx).trim(), desc: summary.slice(parenIdx) };
    }
    return { name: summary, desc: '' };
  }

  $: summaryLabel = (() => {
    const parts: string[] = [];
    if (toolCount > 0) parts.push(`${toolCount} tool ${toolCount === 1 ? 'call' : 'calls'}`);
    if (reasoningCount > 0) parts.push(`${reasoningCount} reasoning ${reasoningCount === 1 ? 'step' : 'steps'}`);
    if (parts.length === 0) parts.push(`${totalSteps} ${totalSteps === 1 ? 'step' : 'steps'}`);
    return parts.join(', ');
  })();

  $: statusLabel = (() => {
    if (live) return 'thinking\u2026';
    if (allComplete) return 'completed';
    return '';
  })();
</script>

{#if totalSteps > 0 || live}
<div class="thinking-envelope" class:blocked={hasBlock} class:has-fail={hasFail} class:expanded class:live>
  <button class="thinking-toggle" on:click={() => expanded = !expanded}>
    <span class="toggle-chevron">
      {#if expanded}
        <ChevronDown size={12} />
      {:else}
        <ChevronRight size={12} />
      {/if}
    </span>
    <span class="toggle-label">
      {summaryLabel}
      {#if statusLabel}
        <span class="toggle-status">{statusLabel}</span>
      {/if}
    </span>
  </button>

  <div class="thinking-steps" class:show={expanded}>
    {#each items as item, i (i)}
      <div class="step-row">
        {#if item.kind === 'reasoning'}
          <span class="step-reasoning">{item.name}</span>
        {:else}
          <span class="step-tool-name" class:failed={item.failed} class:blocked={item.blocked}>{item.name}</span>
          {#if item.desc}
            <span class="step-tool-desc" class:failed={item.failed} class:blocked={item.blocked}>{item.desc}</span>
          {/if}
        {/if}
      </div>
    {/each}
  </div>
</div>
{/if}

<style>
  .thinking-envelope {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    font-style: italic;
    color: var(--text-tertiary);
    margin: 0;
  }

  .thinking-envelope.live {
    color: var(--accent-dim);
  }

  .thinking-envelope.blocked .toggle-label {
    color: var(--error);
  }

  .thinking-toggle {
    padding: 4px 0;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    border: none;
    background: none;
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    font-style: italic;
    color: inherit;
    text-align: left;
    transition: color 150ms ease;
  }

  .thinking-toggle:hover {
    color: var(--text-secondary);
  }

  .toggle-chevron {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    opacity: 0.6;
  }

  .toggle-label {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .toggle-status {
    opacity: 0.6;
    font-size: 10px;
  }

  .thinking-steps {
    max-height: 0;
    overflow: hidden;
    padding: 0 16px;
    transition: max-height 200ms ease, padding 200ms ease;
  }

  .thinking-steps.show {
    max-height: 500px;
    padding: 4px 16px 6px;
  }

  .step-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 0;
    font-size: 11px;
    font-style: italic;
    color: var(--text-tertiary);
  }

  .step-reasoning {
    font-family: 'Exo 2', sans-serif;
  }

  .step-tool-name {
    color: var(--accent-dim);
    opacity: 0.5;
    font-weight: 500;
    flex-shrink: 0;
  }

  .step-tool-name.failed { color: var(--warning); opacity: 0.6; }
  .step-tool-name.blocked { color: var(--error); opacity: 0.6; }

  .step-tool-desc {
    opacity: 0.4;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }

  .step-tool-desc.failed { color: var(--warning); opacity: 0.5; }
  .step-tool-desc.blocked { color: var(--error); opacity: 0.5; }
</style>
