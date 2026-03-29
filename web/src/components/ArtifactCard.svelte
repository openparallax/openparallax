<script lang="ts">
  import type { Artifact } from '../lib/types';

  export let artifact: Artifact;

  function iconForType(type: string): string {
    switch (type) {
      case 'file': return '\uD83D\uDCC4';
      case 'command_output': return '\uD83D\uDCBB';
      default: return '\uD83D\uDCC1';
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
</script>

<div class="artifact-card">
  <div class="artifact-icon">{iconForType(artifact.type)}</div>
  <div class="artifact-name">{artifact.title}</div>
  <div class="artifact-meta">
    {formatSize(artifact.size_bytes)} &middot; {artifact.language || artifact.type}
  </div>
</div>

<style>
  .artifact-card {
    padding: 14px;
    border-radius: var(--radius);
    background: var(--accent-ghost);
    border: 1px solid var(--accent-border);
    margin-bottom: 10px;
    cursor: pointer;
    transition: all 150ms ease;
  }

  .artifact-card:hover {
    border-color: var(--accent-border-active);
    box-shadow: var(--accent-glow);
  }

  .artifact-icon { font-size: 20px; margin-bottom: 8px; }

  .artifact-name {
    font-size: 13px; font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 4px;
  }

  .artifact-meta {
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }
</style>
