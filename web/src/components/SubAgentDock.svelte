<script lang="ts">
  import { subAgents, hasActiveSubAgents, dismissSubAgent, type SubAgentState } from '../stores/subagents';
  import { sendMessage } from '../lib/websocket';
  import { currentSessionId } from '../stores/session';

  let expandedAgent: string | null = null;

  function toggleExpand(name: string) {
    expandedAgent = expandedAgent === name ? null : name;
  }

  function formatElapsed(ms: number | undefined): string {
    if (!ms) return '0s';
    const secs = Math.floor(ms / 1000);
    if (secs < 60) return `${secs}s`;
    const mins = Math.floor(secs / 60);
    const remainSecs = secs % 60;
    return `${mins}m ${remainSecs}s`;
  }

  function statusDotClass(status: SubAgentState['status']): string {
    switch (status) {
      case 'spawning':
      case 'working': return 'dot-working';
      case 'completed': return 'dot-completed';
      case 'failed':
      case 'timed_out': return 'dot-failed';
      case 'cancelled': return 'dot-cancelled';
      default: return '';
    }
  }

  function truncateTask(task: string, max: number = 40): string {
    return task.length > max ? task.slice(0, max) + '...' : task;
  }

  function handleCancel(name: string) {
    const sid = $currentSessionId;
    if (sid) {
      sendMessage(sid, `/cancel-agent ${name}`, 'normal');
    }
  }

  $: agentList = Array.from($subAgents.values());
  $: show = agentList.length > 0;
</script>

{#if show}
  <div class="dock glass" class:expanded={expandedAgent !== null}>
    <div class="dock-header">
      <span class="dock-title">SUB-AGENTS</span>
      <span class="dock-count">{agentList.length}</span>
    </div>
    <div class="pills">
      {#each agentList as agent (agent.name)}
        <button
          class="pill"
          class:active={expandedAgent === agent.name}
          on:click={() => toggleExpand(agent.name)}
        >
          <span class="dot {statusDotClass(agent.status)}"></span>
          <span class="pill-name">{agent.name}</span>
          <span class="pill-task">{truncateTask(agent.task)}</span>
        </button>
      {/each}
    </div>

    {#if expandedAgent}
      {#each agentList.filter(a => a.name === expandedAgent) as agent (agent.name)}
        <div class="detail">
          <div class="detail-row">
            <span class="label">Task</span>
            <span class="value">{agent.task}</span>
          </div>
          <div class="detail-row">
            <span class="label">Status</span>
            <span class="value status-{agent.status}">{agent.status}</span>
          </div>
          {#if agent.toolGroups.length > 0}
            <div class="detail-row">
              <span class="label">Tools</span>
              <span class="value">{agent.toolGroups.join(', ')}</span>
            </div>
          {/if}
          <div class="detail-row">
            <span class="label">LLM Calls</span>
            <span class="value">{agent.llmCalls}</span>
          </div>
          <div class="detail-row">
            <span class="label">Tool Calls</span>
            <span class="value">{agent.toolCalls}</span>
          </div>
          <div class="detail-row">
            <span class="label">Elapsed</span>
            <span class="value">{formatElapsed(agent.elapsedMs)}</span>
          </div>
          {#if agent.result}
            <div class="detail-result">
              <span class="label">Result</span>
              <pre class="result-preview">{agent.result.slice(0, 300)}{agent.result.length > 300 ? '...' : ''}</pre>
            </div>
          {/if}
          {#if agent.error}
            <div class="detail-error">
              <span class="label">Error</span>
              <span class="error-text">{agent.error}</span>
            </div>
          {/if}
          <div class="detail-actions">
            {#if agent.status === 'working' || agent.status === 'spawning'}
              <button class="action-btn cancel" on:click={() => handleCancel(agent.name)}>Cancel</button>
            {:else}
              <button class="action-btn dismiss" on:click={() => dismissSubAgent(agent.name)}>Dismiss</button>
            {/if}
          </div>
        </div>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .dock {
    border-top: 1px solid var(--accent-border);
    padding: 8px 12px;
    animation: slideUp 300ms ease-out;
  }

  @keyframes slideUp {
    from { transform: translateY(100%); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }

  .dock-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }

  .dock-title {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--accent-dim);
    text-transform: uppercase;
  }

  .dock-count {
    font-size: 10px;
    font-family: 'JetBrains Mono', monospace;
    color: var(--accent);
    background: var(--accent-ghost);
    padding: 1px 6px;
    border-radius: 8px;
  }

  .pills {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .pill {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border: none;
    border-radius: 16px;
    background: rgba(12, 16, 28, 0.50);
    color: rgba(255, 255, 255, 0.8);
    font-size: 12px;
    font-family: 'Exo 2', sans-serif;
    cursor: pointer;
    transition: background 200ms;
  }

  .pill:hover, .pill.active {
    background: var(--accent-subtle);
  }

  .pill-name {
    font-weight: 600;
    color: var(--accent);
  }

  .pill-task {
    color: rgba(255, 255, 255, 0.5);
    font-size: 11px;
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-working {
    background: var(--accent);
    animation: pulse 1.5s infinite;
  }

  .dot-completed {
    background: #4caf50;
  }

  .dot-failed {
    background: #f44336;
  }

  .dot-cancelled {
    background: rgba(255, 255, 255, 0.3);
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .detail {
    margin-top: 10px;
    padding: 10px;
    background: rgba(12, 16, 28, 0.40);
    border-radius: 8px;
    border: 1px solid var(--accent-border);
  }

  .detail-row {
    display: flex;
    justify-content: space-between;
    padding: 3px 0;
    font-size: 12px;
  }

  .label {
    color: rgba(255, 255, 255, 0.5);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .value {
    color: rgba(255, 255, 255, 0.85);
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
  }

  .status-completed { color: #4caf50; }
  .status-failed, .status-timed_out { color: #f44336; }
  .status-working, .status-spawning { color: var(--accent); }
  .status-cancelled { color: rgba(255, 255, 255, 0.4); }

  .detail-result, .detail-error {
    margin-top: 6px;
  }

  .result-preview {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: rgba(255, 255, 255, 0.7);
    white-space: pre-wrap;
    word-break: break-word;
    margin: 4px 0 0;
    padding: 6px;
    background: rgba(0, 0, 0, 0.2);
    border-radius: 4px;
    max-height: 100px;
    overflow-y: auto;
  }

  .error-text {
    color: #f44336;
    font-size: 12px;
  }

  .detail-actions {
    margin-top: 8px;
    display: flex;
    gap: 8px;
  }

  .action-btn {
    padding: 4px 12px;
    border: 1px solid var(--accent-border);
    border-radius: 4px;
    background: transparent;
    color: rgba(255, 255, 255, 0.7);
    font-size: 11px;
    cursor: pointer;
    transition: all 200ms;
  }

  .action-btn:hover {
    background: var(--accent-subtle);
    color: white;
  }

  .action-btn.cancel {
    border-color: rgba(244, 67, 54, 0.3);
    color: #f44336;
  }

  .action-btn.cancel:hover {
    background: rgba(244, 67, 54, 0.15);
  }
</style>
