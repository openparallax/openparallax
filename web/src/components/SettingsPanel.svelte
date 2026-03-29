<script lang="ts">
  import { onMount } from 'svelte';
  import { X, User, Cpu, Shield, Brain, Mail, Calendar, Globe, Info } from 'lucide-svelte';
  import { settingsOpen } from '../stores/settings';
  import { getStatus } from '../lib/api';

  let agentName = '';
  let model = '';
  let sessionCount = 0;

  const sections = [
    { id: 'identity', label: 'Agent Identity', icon: User },
    { id: 'llm', label: 'LLM Provider', icon: Cpu },
    { id: 'shield', label: 'Shield', icon: Shield },
    { id: 'memory', label: 'Memory', icon: Brain },
    { id: 'email', label: 'Email', icon: Mail },
    { id: 'calendar', label: 'Calendar', icon: Calendar },
    { id: 'web', label: 'Web UI', icon: Globe },
    { id: 'about', label: 'About', icon: Info },
  ];

  let expandedSection: string | null = 'identity';

  onMount(async () => {
    try {
      const status = await getStatus();
      agentName = status.agent_name;
      model = status.model;
      sessionCount = status.session_count;
    } catch {
      /* engine not ready */
    }
  });

  function close() {
    settingsOpen.set(false);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  function handleBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('settings-backdrop')) {
      close();
    }
  }

  function toggleSection(id: string) {
    expandedSection = expandedSection === id ? null : id;
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $settingsOpen}
  <div class="settings-backdrop" on:click={handleBackdropClick} role="presentation">
    <div class="settings-panel glass-elevated">
      <div class="settings-header">
        <h2 class="settings-title">Settings</h2>
        <button class="close-btn" on:click={close}>
          <X size={16} />
        </button>
      </div>

      <div class="settings-body">
        {#each sections as section (section.id)}
          <div class="settings-section">
            <button class="section-header" on:click={() => toggleSection(section.id)}>
              <svelte:component this={section.icon} size={14} />
              <span>{section.label}</span>
            </button>
            {#if expandedSection === section.id}
              <div class="section-content">
                {#if section.id === 'identity'}
                  <div class="field">
                    <span class="field-label">Agent Name</span>
                    <div class="field-value">{agentName || 'Atlas'}</div>
                  </div>
                {:else if section.id === 'llm'}
                  <div class="field">
                    <span class="field-label">Model</span>
                    <div class="field-value">{model || 'Not configured'}</div>
                  </div>
                {:else if section.id === 'about'}
                  <div class="field">
                    <span class="field-label">Sessions</span>
                    <div class="field-value">{sessionCount}</div>
                  </div>
                  <div class="field">
                    <span class="field-label">License</span>
                    <div class="field-value">Apache 2.0</div>
                  </div>
                {:else}
                  <div class="field">
                    <div class="field-value placeholder">Configuration available in config.yaml</div>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .settings-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
  }

  .settings-panel {
    width: 400px;
    height: 100%;
    display: flex;
    flex-direction: column;
    animation: slide-from-left 300ms ease-out;
    overflow: hidden;
  }

  @keyframes slide-from-left {
    from { transform: translateX(-100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }

  .settings-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid var(--accent-border);
  }

  .settings-title {
    font-family: 'Exo 2', sans-serif;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .close-btn {
    width: 32px; height: 32px;
    border-radius: 6px;
    border: 1px solid var(--accent-border);
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 150ms ease;
  }
  .close-btn:hover {
    border-color: var(--accent-border-active);
    color: var(--text-primary);
  }

  .settings-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .settings-section {
    border-radius: var(--radius);
    border: 1px solid var(--accent-border);
    overflow: hidden;
  }

  .section-header {
    width: 100%;
    padding: 12px 16px;
    display: flex;
    align-items: center;
    gap: 10px;
    border: none;
    background: var(--accent-ghost);
    color: var(--text-primary);
    font-family: 'Exo 2', sans-serif;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: background 150ms ease;
    text-align: left;
  }
  .section-header:hover { background: var(--accent-subtle); }

  .section-content {
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    border-top: 1px solid var(--accent-border);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-label {
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .field-value {
    font-size: 14px;
    color: var(--text-primary);
  }

  .field-value.placeholder {
    color: var(--text-tertiary);
    font-size: 12px;
    font-style: italic;
  }
</style>
