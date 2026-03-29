<script lang="ts">
  import { afterUpdate } from 'svelte';
  import { Search, Plus } from 'lucide-svelte';
  import { messages, pendingToolCalls, streaming, streamingText } from '../stores/messages';
  import { connected } from '../stores/connection';
  import Message from './Message.svelte';
  import ToolCallGroup from './ToolCallGroup.svelte';
  import InputArea from './InputArea.svelte';

  let messagesEl: HTMLDivElement;

  afterUpdate(() => {
    if (messagesEl) {
      messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
    }
  });
</script>

<div class="chat-panel glass" class:streaming={$streaming}>
  <div class="chat-header">
    <div class="chat-header-left">
      <div class="chat-breadcrumb">
        ATLAS <span>&rsaquo;</span> WORKSPACE
      </div>
    </div>
    <div class="chat-header-right">
      {#if $connected}
        <div class="sync-badge">SYNC_LIVE</div>
      {:else}
        <div class="sync-badge disconnected">OFFLINE</div>
      {/if}
      <div class="chat-actions">
        <button class="chat-action-btn"><Search size={14} /></button>
        <button class="chat-action-btn"><Plus size={14} /></button>
      </div>
    </div>
  </div>

  <div class="messages" bind:this={messagesEl}>
    {#each $messages as msg (msg.id)}
      {#if msg.role === 'assistant' && msg.thoughts && msg.thoughts.length > 0}
        <ToolCallGroup thoughts={msg.thoughts} />
      {/if}
      <Message message={msg} />
    {/each}

    {#if $pendingToolCalls.length > 0}
      <ToolCallGroup toolCalls={$pendingToolCalls} live={true} />
    {/if}

    {#if $streaming && $streamingText}
      <Message message={{ id: 'streaming', session_id: '', role: 'assistant', content: $streamingText, timestamp: new Date().toISOString() }} isStreaming={true} />
    {/if}

    {#if $streaming && !$streamingText && $pendingToolCalls.length === 0}
      <div class="streaming-indicator">
        <div class="streaming-dot"></div>
        <div class="streaming-dot"></div>
        <div class="streaming-dot"></div>
      </div>
    {/if}
  </div>

  <InputArea />
</div>

<style>
  .chat-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    margin: 0 1px;
    min-width: 400px;
    transition: border-color 300ms ease, box-shadow 300ms ease;
  }

  .chat-panel.streaming {
    animation: breathe 2.5s ease-in-out infinite;
  }

  .chat-header {
    padding: 14px 20px;
    border-bottom: 1px solid var(--cyan-border);
    display: flex; align-items: center;
    justify-content: space-between;
  }

  .chat-header-left { display: flex; align-items: center; gap: 12px; }
  .chat-header-right { display: flex; align-items: center; gap: 12px; }

  .chat-breadcrumb {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-tertiary);
    letter-spacing: 0.03em;
  }
  .chat-breadcrumb span { color: var(--text-secondary); }

  .sync-badge {
    padding: 4px 12px;
    border-radius: 20px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px; font-weight: 600;
    letter-spacing: 0.05em;
    background: rgba(0, 230, 118, 0.1);
    color: var(--success);
    border: 1px solid rgba(0, 230, 118, 0.2);
  }

  .sync-badge.disconnected {
    background: rgba(255, 61, 90, 0.1);
    color: var(--error);
    border-color: rgba(255, 61, 90, 0.2);
  }

  .chat-actions { display: flex; gap: 8px; }

  .chat-action-btn {
    width: 32px; height: 32px;
    border-radius: var(--radius);
    border: 1px solid var(--cyan-border);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 150ms ease;
  }

  .chat-action-btn:hover {
    border-color: var(--cyan-border-active);
    color: var(--text-primary);
    background: var(--bg-surface-hover);
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
    display: flex; flex-direction: column;
    gap: 16px;
  }

  .streaming-indicator {
    display: flex; gap: 4px;
    padding: 8px 0;
  }

  .streaming-dot {
    width: 5px; height: 5px;
    border-radius: 50%;
    background: var(--cyan-dim);
    animation: stream-pulse 1.4s ease-in-out infinite;
  }
  .streaming-dot:nth-child(2) { animation-delay: 0.2s; }
  .streaming-dot:nth-child(3) { animation-delay: 0.4s; }
</style>
