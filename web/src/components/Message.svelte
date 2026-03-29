<script lang="ts">
  import type { Message as MessageType, Artifact } from '../lib/types';
  import { renderMarkdown } from '../lib/format';
  import { openArtifactTab } from '../stores/artifacts';
  import { activeNavItem } from '../stores/settings';

  export let message: MessageType;
  export let isStreaming = false;
  export let agentName = 'Atlas';
  export let agentAvatar = 'A';

  $: isAtlas = message.role === 'assistant';
  $: htmlContent = renderMarkdown(message.content);
  $: timestamp = new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  $: messageArtifacts = message.artifacts || [];

  function viewArtifact(artifact: Artifact) {
    openArtifactTab(artifact);
    activeNavItem.set('chat');
  }
</script>

<div class="message" class:atlas={isAtlas} class:user={!isAtlas}>
  <div class="msg-header">
    <div class="msg-avatar" class:atlas-avatar={isAtlas} class:user-avatar={!isAtlas}>
      {isAtlas ? agentAvatar : 'Y'}
    </div>
    <div class="msg-name">{isAtlas ? agentName : 'You'}</div>
    <div class="msg-time">{timestamp}</div>
  </div>
  <div class="msg-bubble markdown-content">
    {@html htmlContent}
    {#if isStreaming}
      <span class="cursor"></span>
    {/if}
    {#each messageArtifacts as artifact (artifact.id)}
      <button class="artifact-ref" on:click={() => viewArtifact(artifact)}>
        &#x1F4C4; {artifact.title} &rarr; View in canvas
      </button>
    {/each}
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

  .msg-header {
    display: flex; align-items: center;
    gap: 6px;
    margin-bottom: 4px;
    font-size: 12px;
  }

  .message.atlas .msg-header { padding-left: 2px; }
  .message.user .msg-header { padding-right: 2px; flex-direction: row-reverse; }

  .msg-avatar {
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

  .msg-name { font-weight: 600; }
  .message.atlas .msg-name { color: var(--accent); }
  .message.user .msg-name { color: var(--text-secondary); }

  .msg-time { color: var(--text-tertiary); font-size: 11px; }

  .msg-bubble {
    max-width: 92%;
    padding: 12px 16px;
    border-radius: 6px;
    font-size: 14px;
    line-height: 1.65;
  }

  .message.atlas .msg-bubble {
    background: var(--accent-ghost);
    border: 1px solid var(--accent-border);
    border-left: 2px solid var(--accent-border-active);
  }

  .message.user .msg-bubble {
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

  .artifact-ref {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    background: var(--accent-ghost);
    border: 1px solid var(--accent-border);
    border-radius: 4px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--accent);
    cursor: pointer;
    transition: all 150ms ease;
    margin-top: 8px;
  }

  .artifact-ref:hover {
    border-color: var(--accent-border-active);
    box-shadow: var(--accent-glow);
  }
</style>
