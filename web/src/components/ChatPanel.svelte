<script lang="ts">
  import { afterUpdate } from 'svelte';
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

  $: messageCount = $messages.length;
</script>

<div class="chat glass" class:streaming={$streaming}>
  <div class="chat-header">
    <span>CHAT</span>
    <span class="msg-count">{messageCount} messages</span>
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
      <div class="thinking">
        <div class="thinking-dot"></div>
        <div class="thinking-dot"></div>
        <div class="thinking-dot"></div>
      </div>
    {/if}
  </div>

  <InputArea />
</div>

<style>
  .chat {
    width: 380px;
    min-width: 380px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: border-color 300ms ease, box-shadow 300ms ease;
  }

  .chat.streaming {
    animation: breathe 2.5s ease-in-out infinite;
  }

  .chat-header {
    padding: 12px 16px;
    border-bottom: 1px solid var(--accent-border);
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: var(--text-tertiary);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .msg-count {
    color: var(--accent-dim);
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex; flex-direction: column;
    gap: 14px;
  }

  .thinking {
    display: flex; gap: 4px;
    padding: 6px 0;
  }

  .thinking-dot {
    width: 5px; height: 5px;
    border-radius: 50%;
    background: var(--accent-dim);
    animation: stream-pulse 1.4s ease-in-out infinite;
  }
  .thinking-dot:nth-child(2) { animation-delay: 0.2s; }
  .thinking-dot:nth-child(3) { animation-delay: 0.4s; }

  @media (max-width: 1200px) {
    .chat { width: 340px; min-width: 340px; }
  }

  @media (max-width: 800px) {
    .chat { width: 100%; min-width: 0; }
  }
</style>
