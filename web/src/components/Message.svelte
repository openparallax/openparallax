<script lang="ts">
  import type { Message as MessageType } from '../lib/types';
  import { renderMarkdown } from '../lib/format';

  export let message: MessageType;
  export let isStreaming = false;

  $: isAtlas = message.role === 'assistant';
  $: htmlContent = renderMarkdown(message.content);
  $: timestamp = new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
</script>

<div class="message" class:atlas={isAtlas} class:user={!isAtlas}>
  <div class="message-header">
    <div class="message-avatar" class:atlas-avatar={isAtlas} class:user-avatar={!isAtlas}>
      {isAtlas ? 'A' : 'Y'}
    </div>
    <div class="message-name">{isAtlas ? 'Atlas' : 'You'}</div>
    <div class="message-time">{timestamp}</div>
  </div>
  <div class="message-bubble markdown-content">
    {@html htmlContent}
    {#if isStreaming}
      <span class="cursor"></span>
    {/if}
  </div>
</div>

<style>
  .message {
    display: flex;
    flex-direction: column;
    animation: msg-in 300ms ease-out;
  }

  .message.atlas { align-items: flex-start; }
  .message.user { align-items: flex-end; }

  .message-header {
    display: flex; align-items: center;
    gap: 8px;
    margin-bottom: 4px;
    font-size: 12px;
  }

  .message.atlas .message-header { padding-left: 2px; }
  .message.user .message-header { padding-right: 2px; flex-direction: row-reverse; }

  .message-avatar {
    width: 20px; height: 20px;
    border-radius: 5px;
    display: flex; align-items: center; justify-content: center;
    font-size: 10px; font-weight: 700;
    font-family: 'JetBrains Mono', monospace;
  }

  .atlas-avatar {
    background: var(--accent-subtle);
    color: var(--accent);
    border: 1px solid var(--accent-border-active);
  }

  .user-avatar {
    background: rgba(240, 240, 245, 0.08);
    color: var(--text-secondary);
    border: 1px solid rgba(240, 240, 245, 0.1);
  }

  .message-name { font-weight: 600; }
  .message.atlas .message-name { color: var(--accent); }
  .message.user .message-name { color: var(--text-secondary); }

  .message-time { color: var(--text-tertiary); font-size: 11px; }

  .message-bubble {
    max-width: 92%;
    padding: 12px 16px;
    border-radius: 6px;
    font-size: 14px;
    line-height: 1.65;
  }

  .message.atlas .message-bubble {
    background: var(--accent-ghost);
    border: 1px solid var(--accent-border);
    border-left: 2px solid var(--accent-border-active);
  }

  .message.user .message-bubble {
    background: rgba(240, 240, 245, 0.04);
    border: 1px solid rgba(240, 240, 245, 0.06);
    border-right: 2px solid rgba(240, 240, 245, 0.2);
    max-width: 85%;
  }

  .cursor {
    display: inline-block;
    width: 2px; height: 14px;
    background: var(--accent);
    margin-left: 1px;
    animation: blink 1s infinite;
    vertical-align: text-bottom;
  }
</style>
