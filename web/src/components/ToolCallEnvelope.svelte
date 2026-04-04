<script lang="ts">
  import { ChevronRight, ChevronDown } from 'lucide-svelte';
  import type { ToolCall, Thought } from '../lib/types';

  export let toolCalls: ToolCall[] = [];
  export let thoughts: Thought[] = [];
  export let live = false;

  let expanded = false;

  $: items = thoughts.length > 0 ? thoughts : buildFromToolCalls(toolCalls);
  $: toolCount = items.filter(t => t.stage === 'tool_call').length;
  $: reasoningCount = items.filter(t => t.stage === 'reasoning').length;
  $: totalSteps = items.length;
  $: hasBlock = toolCalls.length > 0
    ? toolCalls.some(tc => tc.shieldVerdict?.decision === 'BLOCK')
    : items.some(t => t.detail?.shield === 'BLOCK');
  $: hasDetailInfo = items.some(t => t.detail?.success !== undefined);
  $: allComplete = !live && (toolCalls.length > 0
    ? toolCalls.every(tc => tc.result)
    : hasDetailInfo);
  $: successCount = toolCalls.length > 0
    ? toolCalls.filter(tc => tc.result?.success).length
    : items.filter(t => t.stage === 'tool_call' && t.detail?.success === true).length;

  function buildFromToolCalls(tcs: ToolCall[]): Thought[] {
    return tcs.map(tc => ({
      stage: 'tool_call' as const,
      summary: `${tc.toolName} \u2014 ${tc.summary}`,
      detail: {
        tool_name: tc.toolName,
        success: tc.result?.success,
        shield: tc.shieldVerdict?.decision,
        result_summary: tc.result?.summary,
      },
    }));
  }

  function parseToolName(thought: Thought): { name: string; desc: string } {
    if (thought.detail?.tool_name) {
      const name = thought.detail.tool_name as string;
      const desc = thought.detail?.result_summary as string || thought.summary || '';
      return { name, desc };
    }
    const summary = thought.summary || '';
    // Server thoughts use formats like "write_file → wrote 743 bytes..." or "load_tools(files)"
    for (const sep of [' \u2014 ', ' — ', ' \u2192 ']) {
      if (summary.includes(sep)) {
        const idx = summary.indexOf(sep);
        return { name: summary.slice(0, idx).trim(), desc: summary.slice(idx + sep.length).trim() };
      }
    }
    // Handle "load_tools(files)" format
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
    if (allComplete && toolCount > 0 && hasDetailInfo) return `${successCount}/${toolCount} succeeded`;
    if (!live && toolCount > 0 && !hasDetailInfo) return 'completed';
    return '';
  })();
</script>

{#if totalSteps > 0 || live}
<div class="thinking-envelope" class:blocked={hasBlock} class:expanded class:live>
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
        {#if item.stage === 'reasoning'}
          <span class="step-reasoning">{item.summary}</span>
        {:else}
          {@const parsed = parseToolName(item)}
          {@const blocked = item.detail?.shield === 'BLOCK'}
          {@const failed = item.detail?.success === false && !blocked}
          <span class="step-tool-name" class:failed class:blocked>{parsed.name}</span>
          {#if parsed.desc}
            <span class="step-tool-desc" class:failed class:blocked>{parsed.desc}</span>
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
    color: var(--text-tertiary);
    text-align: left;
    transition: color 150ms ease;
  }

  .thinking-toggle:hover {
    color: var(--text-secondary);
  }

  .toggle-chevron {
    color: var(--text-tertiary);
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
