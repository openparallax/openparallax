import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  messages, pendingToolCalls, artifacts, shieldLog,
  streaming, streamingText,
  addUserMessage, appendToken, addToolCall, updateToolCallVerdict,
  completeToolCall, addArtifact, finalizeResponse, clearMessages,
  setStreaming, startNewStream,
} from '../stores/messages';
import { artifactTabs, clearArtifactTabs } from '../stores/artifacts';

describe('messages store', () => {
  beforeEach(() => {
    clearMessages();
    clearArtifactTabs();
  });

  it('starts empty', () => {
    expect(get(messages)).toEqual([]);
    expect(get(streaming)).toBe(false);
    expect(get(streamingText)).toBe('');
  });

  it('addUserMessage appends a user message', () => {
    addUserMessage('hello');
    const msgs = get(messages);
    expect(msgs).toHaveLength(1);
    expect(msgs[0].role).toBe('user');
    expect(msgs[0].content).toBe('hello');
  });

  it('appendToken accumulates streaming text', () => {
    appendToken('Hello');
    appendToken(' world');
    expect(get(streamingText)).toBe('Hello world');
  });

  it('startNewStream resets streaming state', () => {
    appendToken('old');
    startNewStream();
    expect(get(streaming)).toBe(true);
    expect(get(streamingText)).toBe('');
  });

  it('addToolCall pushes to pending', () => {
    addToolCall({ id: 't1', toolName: 'read_file', summary: 'Reading', expanded: false });
    expect(get(pendingToolCalls)).toHaveLength(1);
    expect(get(pendingToolCalls)[0].toolName).toBe('read_file');
  });

  it('updateToolCallVerdict attaches verdict and logs', () => {
    addToolCall({ id: 't1', toolName: 'read_file', summary: 'Reading', expanded: false });
    updateToolCallVerdict({
      toolName: 'read_file',
      decision: 'ALLOW',
      tier: 0,
      confidence: 1,
      reasoning: 'safe',
    });

    const calls = get(pendingToolCalls);
    expect(calls[0].shieldVerdict?.decision).toBe('ALLOW');
    expect(get(shieldLog)).toHaveLength(1);
  });

  it('completeToolCall attaches result', () => {
    addToolCall({ id: 't1', toolName: 'write_file', summary: 'Writing', expanded: false });
    completeToolCall({ tool_name: 'write_file', success: true, summary: 'done' });

    const calls = get(pendingToolCalls);
    expect(calls[0].result?.success).toBe(true);
  });

  it('addArtifact pushes to artifacts and opens tab', () => {
    addArtifact({
      id: 'a1', type: 'file', title: 'test.html',
      path: '/test.html', content: '<h1>Hi</h1>',
      language: 'html', size_bytes: 50, preview_type: 'html',
    });

    expect(get(artifacts)).toHaveLength(1);
    expect(get(artifactTabs)).toHaveLength(1);
  });

  it('finalizeResponse creates assistant message', () => {
    appendToken('Hi there');
    finalizeResponse('Hi there');

    const msgs = get(messages);
    expect(msgs).toHaveLength(1);
    expect(msgs[0].role).toBe('assistant');
    expect(msgs[0].content).toBe('Hi there');
    expect(get(streamingText)).toBe('');
  });

  it('clearMessages resets all state', () => {
    addUserMessage('test');
    addArtifact({
      id: 'a1', type: 'file', title: 'test.html',
      path: '/test.html', content: '', language: 'html',
      size_bytes: 0, preview_type: 'html',
    });
    setStreaming(true);

    clearMessages();

    expect(get(messages)).toEqual([]);
    expect(get(artifacts)).toEqual([]);
    expect(get(pendingToolCalls)).toEqual([]);
    expect(get(streaming)).toBe(false);
  });
});
