"""Shield — 3-tier AI security pipeline."""

from __future__ import annotations

from openparallax_shield.bridge import BridgeProcess
from openparallax_shield.types import ActionRequest, Config, Verdict


class Shield:
    """3-tier AI security pipeline for evaluating agent actions.

    Communicates with the Go shield-bridge binary over JSON-RPC.

    Usage::

        from openparallax_shield import Shield, Config, ActionRequest

        shield = Shield(Config(policy_file="policy.yaml"))
        verdict = shield.evaluate(ActionRequest(type="file_write", payload={"path": "/etc/passwd"}))
        print(verdict.decision)  # "BLOCK"
        shield.close()

    Or as a context manager::

        with Shield(Config(policy_file="policy.yaml")) as shield:
            verdict = shield.evaluate(ActionRequest(type="read_file", payload={"path": "readme.md"}))
    """

    def __init__(self, config: Config | None = None) -> None:
        self._bridge = BridgeProcess("shield-bridge")
        if config is not None:
            self.configure(config)

    def configure(self, config: Config) -> None:
        """Initialize the Shield pipeline with the given configuration."""
        self._bridge.call("configure", config.to_dict())

    def evaluate(self, action: ActionRequest) -> Verdict:
        """Evaluate an action through the 3-tier security pipeline.

        Returns a Verdict with decision (ALLOW/BLOCK/ESCALATE), tier, confidence, and reasoning.
        """
        result = self._bridge.call("evaluate", action.to_dict())
        return Verdict.from_dict(result)

    def close(self) -> None:
        """Shut down the bridge process."""
        self._bridge.close()

    def __enter__(self) -> Shield:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
