import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { artifactTabs, activeTabId, activeTab, openArtifactTab, closeArtifactTab, clearArtifactTabs } from '../stores/artifacts';
import type { Artifact } from '../lib/types';

function makeArtifact(id: string, title = 'test.html'): Artifact {
  return {
    id,
    type: 'file',
    title,
    path: `/workspace/${title}`,
    content: '<h1>Test</h1>',
    language: 'html',
    size_bytes: 100,
    preview_type: 'html',
  };
}

describe('artifacts store', () => {
  beforeEach(() => {
    clearArtifactTabs();
  });

  it('starts with empty tabs', () => {
    expect(get(artifactTabs)).toEqual([]);
    expect(get(activeTabId)).toBeNull();
    expect(get(activeTab)).toBeNull();
  });

  it('openArtifactTab adds a tab and focuses it', () => {
    const a = makeArtifact('a1', 'file.html');
    openArtifactTab(a);

    const tabs = get(artifactTabs);
    expect(tabs).toHaveLength(1);
    expect(tabs[0].id).toBe('a1');
    expect(get(activeTabId)).toBe('a1');
  });

  it('openArtifactTab does not duplicate existing tab', () => {
    const a = makeArtifact('a1');
    openArtifactTab(a);
    openArtifactTab(a);

    expect(get(artifactTabs)).toHaveLength(1);
  });

  it('closeArtifactTab removes tab', () => {
    const a = makeArtifact('a1');
    const b = makeArtifact('a2', 'other.html');
    openArtifactTab(a);
    openArtifactTab(b);

    closeArtifactTab('a1');

    const tabs = get(artifactTabs);
    expect(tabs).toHaveLength(1);
    expect(tabs[0].id).toBe('a2');
  });

  it('closeArtifactTab adjusts activeTabId to last remaining', () => {
    const a = makeArtifact('a1');
    const b = makeArtifact('a2');
    openArtifactTab(a);
    openArtifactTab(b);

    closeArtifactTab('a2');
    expect(get(activeTabId)).toBe('a1');
  });

  it('closeArtifactTab sets null when last tab closed', () => {
    const a = makeArtifact('a1');
    openArtifactTab(a);
    closeArtifactTab('a1');

    expect(get(activeTabId)).toBeNull();
    expect(get(activeTab)).toBeNull();
  });

  it('activeTab derived store resolves correctly', () => {
    const a = makeArtifact('a1', 'index.html');
    openArtifactTab(a);

    const tab = get(activeTab);
    expect(tab).not.toBeNull();
    expect(tab!.artifact.title).toBe('index.html');
  });

  it('enforces 6 tab maximum', () => {
    for (let i = 0; i < 8; i++) {
      openArtifactTab(makeArtifact(`a${i}`, `file${i}.html`));
    }

    const tabs = get(artifactTabs);
    expect(tabs).toHaveLength(6);
    expect(tabs[0].id).toBe('a2');
    expect(tabs[5].id).toBe('a7');
  });

  it('clearArtifactTabs resets everything', () => {
    openArtifactTab(makeArtifact('a1'));
    openArtifactTab(makeArtifact('a2'));
    clearArtifactTabs();

    expect(get(artifactTabs)).toEqual([]);
    expect(get(activeTabId)).toBeNull();
  });
});
