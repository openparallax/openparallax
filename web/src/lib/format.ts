import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Configure marked for simple, safe rendering.
marked.setOptions({
  breaks: true,
  gfm: true,
});

// Override link renderer to open links in new tabs.
const renderer = new marked.Renderer();
renderer.link = function ({ href, title, text }) {
  const titleAttr = title ? ` title="${title}"` : '';
  return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer">${text}</a>`;
};
marked.use({ renderer });

export function renderMarkdown(text: string): string {
  const html = marked.parse(text) as string;
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['target'],
  });
}

export function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function formatRelativeTime(iso: string | undefined): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '';
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 0) return '';
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days > 365) return '';
  return `${days}d ago`;
}
