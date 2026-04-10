/** Shield pipeline configuration. */
export interface ShieldConfig {
  PolicyFile?: string;
  OnnxThreshold?: number;
  HeuristicEnabled?: boolean;
  ClassifierAddr?: string;
  FailClosed?: boolean;
  RateLimit?: number;
  VerdictTTL?: number;
  DailyBudget?: number;
  CanaryToken?: string;
  PromptPath?: string;
  Evaluator?: EvaluatorConfig;
}

/** Tier 2 LLM evaluator configuration. */
export interface EvaluatorConfig {
  Provider: string;
  Model: string;
  APIKeyEnv: string;
  BaseURL?: string;
}

/** An action to evaluate through the Shield pipeline. */
export interface ActionRequest {
  Type: string;
  Payload: Record<string, unknown>;
  Hash?: string;
  MinTier?: number;
}

/** Shield evaluation result. */
export interface Verdict {
  decision: string;
  tier: number;
  confidence: number;
  reasoning: string;
  action_hash: string;
  evaluated_at: string;
  expires_at: string;
}
