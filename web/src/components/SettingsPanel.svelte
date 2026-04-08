<script lang="ts">
  import { onMount } from 'svelte';
  import { X, User, Cpu, Shield, Brain, Mail, Calendar, Globe, Info, Loader2, Lock } from 'lucide-svelte';
  import { settingsOpen } from '../stores/settings';
  import { getSettings } from '../lib/api';

  let settings: Record<string, any> = {};
  let loading = true;

  const sections = [
    { id: 'identity', label: 'Agent Identity', icon: User },
    { id: 'chat', label: 'Chat Model', icon: Cpu },
    { id: 'shield', label: 'Shield', icon: Shield },
    { id: 'memory', label: 'Memory', icon: Brain },
    { id: 'email', label: 'Email', icon: Mail },
    { id: 'calendar', label: 'Calendar', icon: Calendar },
    { id: 'web', label: 'Web UI', icon: Globe },
    { id: 'about', label: 'About', icon: Info },
  ];

  let expandedSection: string | null = 'identity';

  onMount(async () => {
    await loadSettings();
  });

  async function loadSettings() {
    loading = true;
    try {
      settings = await getSettings();
    } catch {
      /* engine not ready */
    }
    loading = false;
  }

  function close() { settingsOpen.set(false); }
  function handleKeydown(e: KeyboardEvent) { if (e.key === 'Escape') close(); }
  function handleBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('settings-backdrop')) close();
  }
  function toggleSection(id: string) { expandedSection = expandedSection === id ? null : id; }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $settingsOpen}
  <div class="settings-backdrop" on:click={handleBackdropClick} role="presentation">
    <div class="settings-panel glass-elevated">
      <div class="settings-header">
        <h2 class="settings-title">Settings</h2>
        <div class="header-actions">
          <button class="close-btn" on:click={close}><X size={16} /></button>
        </div>
      </div>

      <div class="readonly-banner">
        <Lock size={12} />
        <span>Read-only. Use <code>/config set</code> or <code>/model</code> in chat to edit.</span>
      </div>

      <div class="settings-body">
        {#if loading}
          <div class="loading"><Loader2 size={20} class="spin" /></div>
        {:else}
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
                      <span class="field-label">AGENT NAME</span>
                      <div class="field-value">{settings.agent?.name || '—'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">AVATAR</span>
                      <div class="field-value">{settings.agent?.avatar || '—'}</div>
                    </div>

                  {:else if section.id === 'chat'}
                    <div class="field">
                      <span class="field-label">PROVIDER</span>
                      <div class="field-value">{settings.chat?.provider || '—'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">MODEL</span>
                      <div class="field-value">{settings.chat?.model || '—'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">API KEY</span>
                      <div class="field-value">{settings.chat?.api_key_configured ? '✓ Configured' : '✗ Not set'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">BASE URL</span>
                      <div class="field-value">{settings.chat?.base_url || 'Default'}</div>
                    </div>

                  {:else if section.id === 'shield'}
                    <div class="field">
                      <span class="field-label">POLICY</span>
                      <div class="field-value">{settings.shield?.policy || 'default'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">EVALUATOR PROVIDER</span>
                      <div class="field-value">{settings.shield?.evaluator?.provider || '—'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">EVALUATOR MODEL</span>
                      <div class="field-value">{settings.shield?.evaluator?.model || '—'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">TIER 2 BUDGET</span>
                      <div class="field-value">{settings.shield?.tier2_budget || 0}/day</div>
                    </div>
                    <div class="field">
                      <span class="field-label">TIER 2 USED TODAY</span>
                      <div class="field-value">{settings.shield?.tier2_used_today || 0}</div>
                    </div>

                  {:else if section.id === 'memory'}
                    <div class="field">
                      <span class="field-label">EMBEDDING PROVIDER</span>
                      <div class="field-value">{settings.memory?.embedding?.provider || 'None'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">EMBEDDING MODEL</span>
                      <div class="field-value">{settings.memory?.embedding?.model || '—'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">API KEY</span>
                      <div class="field-value">{settings.memory?.embedding?.api_key_configured ? '✓ Configured' : '✗ Not set'}</div>
                    </div>

                  {:else if section.id === 'email'}
                    <div class="field">
                      <span class="field-label">STATUS</span>
                      <div class="field-value">{settings.email?.configured ? '✓ Configured' : 'Not configured'}</div>
                    </div>
                    {#if settings.email?.configured}
                      <div class="field">
                        <span class="field-label">FROM</span>
                        <div class="field-value">{settings.email?.from || ''}</div>
                      </div>
                    {/if}

                  {:else if section.id === 'calendar'}
                    <div class="field">
                      <span class="field-label">STATUS</span>
                      <div class="field-value">{settings.calendar?.configured ? `✓ ${settings.calendar.provider}` : 'Not configured'}</div>
                    </div>

                  {:else if section.id === 'web'}
                    <div class="field">
                      <span class="field-label">PORT</span>
                      <div class="field-value">{settings.web?.port || 3100}</div>
                    </div>

                  {:else if section.id === 'about'}
                    <div class="field">
                      <span class="field-label">SANDBOX</span>
                      <div class="field-value">{settings.sandbox?.active ? `✓ ${settings.sandbox.mode}` : '✗ Inactive'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">MCP SERVERS</span>
                      <div class="field-value">{settings.mcp?.servers?.length || 0} configured</div>
                    </div>
                    <div class="field">
                      <span class="field-label">LICENSE</span>
                      <div class="field-value">Apache 2.0</div>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        {/if}
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

  .header-actions {
    display: flex;
    gap: 8px;
    align-items: center;
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

  .readonly-banner {
    padding: 8px 24px;
    background: var(--accent-ghost);
    border-bottom: 1px solid var(--accent-border);
    color: var(--text-secondary);
    font-size: 11px;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .readonly-banner code {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    padding: 1px 4px;
    background: rgba(12, 16, 28, 0.5);
    border-radius: 3px;
    color: var(--accent);
  }

  .loading {
    display: flex;
    justify-content: center;
    padding: 40px;
    color: var(--text-tertiary);
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
    font-size: 10px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.06em;
  }

  .field-value {
    font-size: 13px;
    color: var(--text-primary);
    font-family: 'JetBrains Mono', monospace;
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
