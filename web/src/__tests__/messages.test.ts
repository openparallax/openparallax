import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  messages, pendingSteps, artifacts, shieldLog,
  streaming, streamingText,
  addUserMessage, appendToken, addToolCallWithFlush, updateToolCallVerdict,
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

  it('addToolCallWithFlush flushes reasoning and adds tool in order', () => {
    appendToken('thinking about this...');
    addToolCallWithFlush('read_file', 'Reading');
    const steps = get(pendingSteps);
    expect(steps).toHaveLength(2);
    expect(steps[0].type).toBe('reasoning');
    expect(steps[0].content).toBe('thinking about this...');
    expect(steps[1].type).toBe('tool_call');
    expect(steps[1].toolName).toBe('read_file');
    expect(get(streamingText)).toBe('');
  });

  it('addToolCallWithFlush skips empty reasoning', () => {
    addToolCallWithFlush('read_file', 'Reading');
    const steps = get(pendingSteps);
    expect(steps).toHaveLength(1);
    expect(steps[0].type).toBe('tool_call');
  });

  it('updateToolCallVerdict attaches verdict and logs', () => {
    addToolCallWithFlush('read_file', 'Reading');
    updateToolCallVerdict({
      toolName: 'read_file',
      decision: 'ALLOW',
      tier: 0,
      confidence: 1,
      reasoning: 'safe',
    });

    const steps = get(pendingSteps);
    expect(steps[0].shieldVerdict?.decision).toBe('ALLOW');
    expect(get(shieldLog)).toHaveLength(1);
  });

  it('completeToolCall attaches result', () => {
    addToolCallWithFlush('write_file', 'Writing');
    completeToolCall({ tool_name: 'write_file', success: true, summary: 'done' });

    const steps = get(pendingSteps);
    const tool = steps.find(s => s.type === 'tool_call');
    expect(tool?.result?.success).toBe(true);
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
    expect(get(pendingSteps)).toEqual([]);
    expect(get(streaming)).toBe(false);
  });

  it('blocked tool call preserves shield info through finalization', () => {
    addToolCallWithFlush('execute_command', 'rm -rf /');
    updateToolCallVerdict({
      toolName: 'execute_command',
      decision: 'BLOCK',
      tier: 1,
      confidence: 0.95,
      reasoning: 'dangerous command',
    });
    completeToolCall({ tool_name: 'execute_command', success: false, summary: 'Blocked: dangerous command' });

    finalizeResponse('I cannot do that.');

    const msgs = get(messages);
    const thoughts = msgs[0].thoughts;
    expect(thoughts).toBeDefined();
    const toolThought = thoughts!.find(t => t.stage === 'tool_call');
    expect(toolThought?.detail?.shield).toBe('BLOCK');
    expect(toolThought?.detail?.success).toBe(false);
  });

  it('interleaves reasoning and tool calls chronologically', () => {
    appendToken('Let me think...');
    addToolCallWithFlush('read_file', 'reading config');
    appendToken('Now I will write...');
    addToolCallWithFlush('write_file', 'writing output');

    const steps = get(pendingSteps);
    expect(steps).toHaveLength(4);
    expect(steps[0].type).toBe('reasoning');
    expect(steps[1].type).toBe('tool_call');
    expect(steps[1].toolName).toBe('read_file');
    expect(steps[2].type).toBe('reasoning');
    expect(steps[3].type).toBe('tool_call');
    expect(steps[3].toolName).toBe('write_file');
  });
});
