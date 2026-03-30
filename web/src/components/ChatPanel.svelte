<script lang="ts">
  import { afterUpdate } from 'svelte';
  import { ChevronDown } from 'lucide-svelte';
  import { messages, pendingToolCalls, streaming, streamingText } from '../stores/messages';
  import { logEntries } from '../stores/console';
  import Message from './Message.svelte';
  import ToolCallEnvelope from './ToolCallEnvelope.svelte';
  import InputArea from './InputArea.svelte';

  let messagesEl: HTMLDivElement;
  let prevMessageCount = 0;
  let showScrollBtn = false;

  afterUpdate(() => {
    if (!messagesEl) return;
    const count = $messages.length;
    const delta = count - prevMessageCount;
    if (delta > 2) {
      messagesEl.scrollTop = messagesEl.scrollHeight;
    } else if (delta > 0 || $streaming) {
      messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
    }
    prevMessageCount = count;
  });

  function handleScroll() {
    if (!messagesEl) return;
    const distFromBottom = messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight;
    showScrollBtn = distFromBottom > 200;
  }

  function scrollToBottom() {
    if (!messagesEl) return;
    messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
  }

  $: messageCount = $messages.length;

  $: totalTokens = $logEntries
    .filter(e => e.event && e.event.includes('llm'))
    .reduce((sum, e) => {
      const d = e.data || {};
      return sum + (Number(d.input_tokens) || 0) + (Number(d.output_tokens) || 0);
    }, 0);

  function formatTokens(n: number): string {
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return String(n);
  }
</script>

<div class="chat glass" class:streaming={$streaming}>
  <div class="chat-header">
    <span>CHAT</span>
    <span class="msg-count">{messageCount} messages{#if totalTokens > 0} &middot; {formatTokens(totalTokens)} tokens{/if}</span>
  </div>

  <div class="messages-container">
    <div class="messages" bind:this={messagesEl} on:scroll={handleScroll}>
      {#each $messages as msg (msg.id)}
        {#if msg.role === 'assistant' && msg.thoughts && msg.thoughts.length > 0}
          <ToolCallEnvelope thoughts={msg.thoughts} />
        {/if}
        <Message message={msg} />
      {/each}

      {#if $pendingToolCalls.length > 0}
        <ToolCallEnvelope toolCalls={$pendingToolCalls} live={true} />
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

    {#if showScrollBtn}
      <button class="scroll-to-bottom" on:click={scrollToBottom} aria-label="Scroll to bottom">
        <ChevronDown size={16} />
      </button>
    {/if}
  </div>

  <InputArea />
</div>

<style>
  .chat {
    width: 100%;
    height: 100%;
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
    flex-shrink: 0;
  }

  .msg-count {
    color: var(--accent-dim);
  }

  .messages-container {
    flex: 1;
    position: relative;
    overflow: hidden;
  }

  .messages {
    position: absolute;
    inset: 0;
    overflow-y: auto;
    padding: 16px;
    display: flex; flex-direction: column;
    gap: 14px;
  }

  .scroll-to-bottom {
    position: absolute;
    bottom: 12px;
    left: 50%;
    transform: translateX(-50%);
    width: 32px; height: 32px;
    border-radius: 50%;
    border: 1px solid var(--accent-border-active);
    background: rgba(12, 16, 28, 0.92);
    backdrop-filter: blur(12px);
    color: var(--accent);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: var(--accent-glow);
    transition: all 150ms ease;
    z-index: 5;
  }

  .scroll-to-bottom:hover {
    background: var(--accent-subtle);
    transform: translateX(-50%) translateY(-1px);
    box-shadow: var(--accent-glow-strong);
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
