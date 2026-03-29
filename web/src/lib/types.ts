export interface Session {
  id: string;
  mode: 'normal' | 'otr';
  title: string;
  created_at: string;
  last_msg_at: string | null;
  message_count: number;
}

export interface Message {
  id: string;
  session_id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  toolCalls?: ToolCall[];
}

export interface ToolCall {
  id: string;
  toolName: string;
  summary: string;
  shieldVerdict?: ShieldVerdict;
  result?: { success: boolean; summary: string };
  expanded: boolean;
}

export interface ShieldVerdict {
  toolName: string;
  decision: 'ALLOW' | 'BLOCK' | 'ESCALATE';
  tier: number;
  confidence: number;
  reasoning: string;
}

export interface Artifact {
  id: string;
  type: string;
  title: string;
  path: string;
  content: string;
  language: string;
  size_bytes: number;
  preview_type: string;
}

export interface WSEvent {
  type: string;
  session_id: string;
  message_id: string;
  text?: { text: string };
  action_started?: { tool_name: string; summary: string };
  shield_verdict?: ShieldVerdict;
  action_completed?: { tool_name: string; success: boolean; summary: string };
  action_artifact?: { artifact: Artifact };
  response_complete?: { content: string };
  otr_blocked?: { reason: string };
  error?: { code: string; message: string };
}

export interface StatusResponse {
  agent_name: string;
  model: string;
  session_count: number;
}
