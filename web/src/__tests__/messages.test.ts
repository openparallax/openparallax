import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  messages, pendingSteps, shieldLog,
  streaming, streamingText,
  addUserMessage, appendToken, addToolCall, updateToolCallVerdict,
  completeToolCall, finalizeResponse, clearMessages,
  setStreaming, startNewStream,
} from '../stores/messages';

describe('messages store', () => {
  beforeEach(() => {
    clearMessages();
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

  it('addToolCall inserts a separator into streamingText and pushes the tool', () => {
    appendToken('thinking about this...');
    addToolCall('read_file', 'Reading');
    const steps = get(pendingSteps);
    expect(steps).toHaveLength(1);
    expect(steps[0].type).toBe('tool_call');
    expect(steps[0].toolName).toBe('read_file');
    // streamingText is preserved (reasoning stays visible) with a
    // blank-line separator appended for the next reasoning burst.
    expect(get(streamingText)).toBe('thinking about this...\n\n');
  });

  it('addToolCall does not double-separate when streamingText already ends in a blank line', () => {
    appendToken('thinking...\n\n');
    addToolCall('read_file', 'Reading');
    expect(get(streamingText)).toBe('thinking...\n\n');
  });

  it('addToolCall on empty stream just adds the tool', () => {
    addToolCall('read_file', 'Reading');
    const steps = get(pendingSteps);
    expect(steps).toHaveLength(1);
    expect(steps[0].type).toBe('tool_call');
    expect(get(streamingText)).toBe('');
  });

  it('updateToolCallVerdict attaches verdict and logs', () => {
    addToolCall('read_file', 'Reading');
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
    addToolCall('write_file', 'Writing');
    completeToolCall({ tool_name: 'write_file', success: true, summary: 'done' });

    const steps = get(pendingSteps);
    expect(steps[0].result?.success).toBe(true);
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
    setStreaming(true);

    clearMessages();

    expect(get(messages)).toEqual([]);
    expect(get(pendingSteps)).toEqual([]);
    expect(get(streaming)).toBe(false);
  });

  it('blocked tool call preserves shield info through finalization', () => {
    addToolCall('execute_command', 'rm -rf /');
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

  it('reasoning fragments accumulate in streamingText across tool calls', () => {
    appendToken('Let me think...');
    addToolCall('read_file', 'reading config');
    appendToken('Now I will write...');
    addToolCall('write_file', 'writing output');

    // Two tool cards collected, no reasoning entries.
    const steps = get(pendingSteps);
    expect(steps).toHaveLength(2);
    expect(steps[0].toolName).toBe('read_file');
    expect(steps[1].toolName).toBe('write_file');

    // Reasoning stays visible in the bubble with separators.
    expect(get(streamingText)).toBe('Let me think...\n\nNow I will write...\n\n');
  });

  it('finalizeResponse uses engine content verbatim and filters reasoning out of thoughts', () => {
    appendToken('thinking');
    addToolCall('read_file', 'reading');
    completeToolCall({ tool_name: 'read_file', success: true, summary: 'ok' });

    // Engine sends merged content + a thoughts list including a
    // reasoning entry. The persisted message must use the merged
    // content as-is and drop reasoning from thoughts.
    finalizeResponse('thinking\n\nDone reading. The answer is 42.', [
      { stage: 'reasoning', summary: 'thinking' },
      { stage: 'tool_call', summary: 'read_file → ok', detail: { tool_name: 'read_file', success: true } },
    ]);

    const msg = get(messages)[0];
    expect(msg.content).toBe('thinking\n\nDone reading. The answer is 42.');
    expect(msg.thoughts).toHaveLength(1);
    expect(msg.thoughts![0].stage).toBe('tool_call');
  });
});
