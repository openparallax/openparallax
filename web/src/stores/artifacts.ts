import { writable, derived } from 'svelte/store';
import type { Artifact } from '../lib/types';

export interface ArtifactTab {
  id: string;
  artifact: Artifact;
}

const MAX_TABS = 6;

export const artifactTabs = writable<ArtifactTab[]>([]);
export const activeTabId = writable<string | null>(null);

export const activeTab = derived(
  [artifactTabs, activeTabId],
  ([$tabs, $id]) => $tabs.find(t => t.id === $id) || null,
);

export function openArtifactTab(artifact: Artifact) {
  artifactTabs.update(tabs => {
    if (tabs.some(t => t.id === artifact.id)) {
      activeTabId.set(artifact.id);
      return tabs;
    }
    let next = [...tabs, { id: artifact.id, artifact }];
    if (next.length > MAX_TABS) {
      next = next.slice(next.length - MAX_TABS);
    }
    activeTabId.set(artifact.id);
    return next;
  });
}

export function closeArtifactTab(id: string) {
  artifactTabs.update(tabs => {
    const filtered = tabs.filter(t => t.id !== id);
    activeTabId.update(current => {
      if (current === id) {
        return filtered.length > 0 ? filtered[filtered.length - 1].id : null;
      }
      return current;
    });
    return filtered;
  });
}

export function clearArtifactTabs() {
  artifactTabs.set([]);
  activeTabId.set(null);
}
