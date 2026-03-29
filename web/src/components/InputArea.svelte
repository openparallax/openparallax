<script lang="ts">
  import { Send, Square } from 'lucide-svelte';
  import { currentSessionId, currentMode, sessions } from '../stores/session';
  import { streaming, addUserMessage } from '../stores/messages';
  import { connected } from '../stores/connection';
  import { sendMessage } from '../lib/websocket';
  import { createSession } from '../lib/api';

  let text = '';
  let textarea: HTMLTextAreaElement;

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  async function handleSend() {
    const content = text.trim();
    if (!content || !$connected) return;

    // Auto-create session if none exists.
    let sid = $currentSessionId;
    if (!sid) {
      try {
        const sess = await createSession($currentMode);
        sessions.update(s => [sess, ...s]);
        currentSessionId.set(sess.id);
        sid = sess.id;
      } catch {
        return;
      }
    }

    addUserMessage(content);
    sendMessage(sid, content, $currentMode);
    text = '';

    if (textarea) {
      textarea.style.height = 'auto';
    }
  }

  function autoResize() {
    if (!textarea) return;
    textarea.style.height = 'auto';
    textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px';
  }

  $: isOTR = $currentMode === 'otr';
</script>

<div class="input-area">
  <div class="input-container">
    <textarea
      bind:this={textarea}
      bind:value={text}
      on:keydown={handleKeydown}
      on:input={autoResize}
      class="input-field"
      class:otr={isOTR}
      placeholder={isOTR ? '[OTR] Type a message...' : 'Type a message...'}
      rows="1"
      disabled={!$connected}
    ></textarea>
    <button class="send-btn" on:click={handleSend} disabled={!$connected || (!text.trim() && !$streaming)}>
      {#if $streaming}
        <Square size={16} />
      {:else}
        <Send size={16} />
      {/if}
    </button>
  </div>
  <div class="input-footer">
    <span>Shift+Enter for multiline</span>
    <span class="encrypted-badge">&#x1F512; ENCRYPTED</span>
  </div>
</div>

<style>
  .input-area {
    padding: 14px 18px;
    border-top: 1px solid var(--cyan-border);
  }

  .input-container {
    display: flex; gap: 10px;
    align-items: flex-end;
  }

  .input-field {
    flex: 1;
    background: rgba(12, 16, 28, 0.6);
    backdrop-filter: blur(12px);
    border: 1px solid var(--cyan-border);
    border-radius: var(--radius);
    padding: 12px 16px;
    color: var(--text-primary);
    font-family: 'Inter', sans-serif;
    font-size: 14px; line-height: 1.5;
    resize: none; outline: none;
    min-height: 44px; max-height: 120px;
    transition: border-color 200ms ease;
  }

  .input-field::placeholder { color: var(--text-tertiary); }

  .input-field:focus {
    border-color: var(--cyan-border-active);
    box-shadow: var(--cyan-glow);
  }

  .input-field.otr {
    border-color: var(--warning-dim);
  }
  .input-field.otr:focus {
    border-color: var(--warning);
    box-shadow: 0 0 20px rgba(255, 171, 0, 0.12);
  }

  .send-btn {
    width: 44px; height: 44px;
    border-radius: var(--radius);
    border: none;
    background: linear-gradient(135deg, var(--cyan), rgba(0, 180, 220, 1));
    color: #06060c;
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 200ms ease;
    box-shadow: 0 0 15px rgba(0, 220, 255, 0.2);
    flex-shrink: 0;
  }

  .send-btn:hover:not(:disabled) {
    box-shadow: 0 0 25px rgba(0, 220, 255, 0.35);
    transform: translateY(-1px);
  }

  .send-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .input-footer {
    display: flex; justify-content: space-between;
    align-items: center;
    padding: 8px 4px 0;
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }

  .encrypted-badge {
    display: flex; align-items: center; gap: 4px;
  }
</style>
