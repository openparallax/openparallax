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

  // Tool call display item. Reasoning text lives in the assistant
  // message bubble itself, not in this dropdown.
  interface DisplayItem {
    name: string;
    desc: string;
    blocked: boolean;
    failed: boolean;
  }

  $: items = buildItems(thoughts, steps);
  $: toolCount = items.length;
  $: totalSteps = items.length;
  $: hasBlock = items.some(t => t.blocked);
  $: hasFail = items.some(t => t.failed);
  $: allComplete = !live && items.length > 0;
  $: successCount = items.filter(t => !t.blocked && !t.failed).length;

  function buildItems(th: Thought[], st: PipelineStep[]): DisplayItem[] {
    // Live mode: build from pipeline steps (already in chronological order).
    if (st.length > 0) {
      return st.map(s => {
        const blocked = s.shieldVerdict?.decision === 'BLOCK';
        const failed = !blocked && s.result?.success === false;
        return {
          name: s.toolName || '',
          desc: s.result?.summary || s.summary || '',
          blocked,
          failed,
        };
      });
    }
    // Finalized mode: build from thoughts (already in order from server).
    // Filter out reasoning entries — they belong in the message bubble.
    if (th.length > 0) {
      return th
        .filter(t => t.stage === 'tool_call')
        .map(t => {
          const blocked = t.detail?.shield === 'BLOCK';
          const failed = !blocked && t.detail?.success === false;
          const parsed = parseToolName(t);
          return { name: parsed.name, desc: parsed.desc, blocked, failed };
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

  $: summaryLabel = toolCount > 0
    ? `${toolCount} tool ${toolCount === 1 ? 'call' : 'calls'}`
    : '0 tool calls';

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
        <span class="step-tool-name" class:failed={item.failed} class:blocked={item.blocked}>{item.name}</span>
        {#if item.desc}
          <span class="step-tool-desc" class:failed={item.failed} class:blocked={item.blocked}>{item.desc}</span>
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
