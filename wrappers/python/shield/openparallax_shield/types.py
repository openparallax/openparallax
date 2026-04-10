"""Data types for the Shield Python wrapper."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class Config:
    """Shield pipeline configuration."""

    policy_file: str = ""
    onnx_threshold: float = 0.85
    heuristic_enabled: bool = True
    classifier_addr: str = ""
    fail_closed: bool = True
    rate_limit: int = 100
    verdict_ttl: int = 60
    daily_budget: int = 100
    canary_token: str = ""
    prompt_path: str = ""
    evaluator: EvaluatorConfig | None = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to JSON-serializable dict for the bridge."""
        d: dict[str, Any] = {
            "PolicyFile": self.policy_file,
            "OnnxThreshold": self.onnx_threshold,
            "HeuristicEnabled": self.heuristic_enabled,
            "FailClosed": self.fail_closed,
            "RateLimit": self.rate_limit,
            "VerdictTTL": self.verdict_ttl,
            "DailyBudget": self.daily_budget,
        }
        if self.classifier_addr:
            d["ClassifierAddr"] = self.classifier_addr
        if self.canary_token:
            d["CanaryToken"] = self.canary_token
        if self.prompt_path:
            d["PromptPath"] = self.prompt_path
        if self.evaluator:
            d["Evaluator"] = self.evaluator.to_dict()
        return d


@dataclass
class EvaluatorConfig:
    """Tier 2 LLM evaluator configuration."""

    provider: str = ""
    model: str = ""
    api_key_env: str = ""
    base_url: str = ""

    def to_dict(self) -> dict[str, Any]:
        """Convert to JSON-serializable dict."""
        d: dict[str, Any] = {
            "Provider": self.provider,
            "Model": self.model,
            "APIKeyEnv": self.api_key_env,
        }
        if self.base_url:
            d["BaseURL"] = self.base_url
        return d


@dataclass
class ActionRequest:
    """An action to evaluate through the Shield pipeline."""

    type: str
    payload: dict[str, Any] = field(default_factory=dict)
    hash: str = ""
    min_tier: int = 0

    def to_dict(self) -> dict[str, Any]:
        """Convert to JSON-serializable dict for the bridge."""
        d: dict[str, Any] = {
            "Type": self.type,
            "Payload": self.payload,
        }
        if self.hash:
            d["Hash"] = self.hash
        if self.min_tier:
            d["MinTier"] = self.min_tier
        return d


@dataclass
class Verdict:
    """Shield evaluation result."""

    decision: str  # "ALLOW", "BLOCK", or "ESCALATE"
    tier: int
    confidence: float
    reasoning: str
    action_hash: str = ""
    evaluated_at: str = ""
    expires_at: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> Verdict:
        """Create a Verdict from the bridge response dict."""
        return cls(
            decision=data.get("decision", "BLOCK"),
            tier=data.get("tier", 0),
            confidence=data.get("confidence", 0.0),
            reasoning=data.get("reasoning", ""),
            action_hash=data.get("action_hash", ""),
            evaluated_at=data.get("evaluated_at", ""),
            expires_at=data.get("expires_at", ""),
        )

    @property
    def allowed(self) -> bool:
        """Return True if the action was allowed."""
        return self.decision == "ALLOW"

    @property
    def blocked(self) -> bool:
        """Return True if the action was blocked."""
        return self.decision == "BLOCK"
