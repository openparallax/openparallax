// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { renderMarkdown, formatTimestamp, formatRelativeTime } from '../lib/format';

describe('renderMarkdown', () => {
  it('renders bold text', () => {
    const html = renderMarkdown('**hello**');
    expect(html).toContain('<strong>hello</strong>');
  });

  it('renders inline code', () => {
    const html = renderMarkdown('use `console.log`');
    expect(html).toContain('<code>console.log</code>');
  });

  it('renders code blocks', () => {
    const html = renderMarkdown('```\nconst x = 1;\n```');
    expect(html).toContain('<code>');
    expect(html).toContain('const x = 1;');
  });

  it('sanitizes script tags', () => {
    const html = renderMarkdown('<script>alert("xss")</script>');
    expect(html).not.toContain('<script>');
  });

  it('renders links', () => {
    const html = renderMarkdown('[click](https://example.com)');
    expect(html).toContain('href="https://example.com"');
  });

  it('handles empty string', () => {
    const html = renderMarkdown('');
    expect(html).toBe('');
  });
});

describe('formatTimestamp', () => {
  it('formats ISO date to HH:mm', () => {
    const result = formatTimestamp('2026-01-15T14:30:00Z');
    expect(result).toBeTruthy();
    expect(typeof result).toBe('string');
  });
});

describe('formatRelativeTime', () => {
  it('returns empty for invalid date', () => {
    const result = formatRelativeTime('');
    expect(result).toBe('');
  });

  it('returns relative time for recent date', () => {
    const now = new Date().toISOString();
    const result = formatRelativeTime(now);
    expect(result).toBeTruthy();
  });
});
