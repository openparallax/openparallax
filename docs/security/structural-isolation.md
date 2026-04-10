# Structural Isolation

These mechanisms make entire categories of attack impossible by construction. They don't check or filter — they structurally prevent the LLM-reasoning process from having the capabilities needed to cause harm.

## Cognitive-Executive Separation

**Threat:** A fully jailbroken or compromised LLM directly executing harmful actions (file deletion, credential theft, network exfiltration).

**Defense:** The agent process owns the LLM session and can only *propose* tool calls over gRPC. The engine process owns execution. The agent has no executors, no file writers, no shell, no HTTP client for tools. Even a fully compromised agent can only send `ToolCallProposed` messages — it cannot bypass the engine's evaluation pipeline.

This is the foundational guarantee. Every other security mechanism is a defense-in-depth layer on top of this structural separation.

**Relevant threats:**
- OWASP LLM06: Excessive Agency
- MITRE ATLAS T0048: External Harms
- CWE-269: Improper Privilege Management

**Code:** `cmd/agent/internal_agent.go` (agent process), `cmd/agent/internal_engine.go` (engine process), `internal/engine/engine_pipeline.go` (gRPC stream handler)

**Non-negotiable.** There is no configuration to merge the agent and engine processes.

## Kernel Sandbox

**Threat:** The agent process accessing files, network, or processes outside its designated scope — either through a bug in the agent code or through a prompt injection that tricks the agent into performing unauthorized actions through mechanisms other than the tool-call protocol.

**Defense:** The agent process is sandboxed at the kernel level:

| Platform | Mechanism | Filesystem | Network | Process Spawn |
|---|---|---|---|---|
| Linux 5.13+ | Landlock LSM | Read-only workspace | LLM API + gRPC only (v4+, kernel 6.7+) | Blocked |
| macOS | sandbox-exec | Read-only workspace | Blocked | Blocked |
| Windows | Job Objects | No restriction | No restriction | Blocked |

The sandbox is best-effort: if the kernel doesn't support it (old kernel, unsupported platform), the agent starts without it but logs the gap. The sandbox cannot be disabled via configuration.

A healthcare researcher reading patient data can trust that even if the LLM is tricked into attempting to exfiltrate records, the sandbox prevents the agent process from opening network connections to unauthorized destinations.

**Relevant threats:**
- OWASP A01: Broken Access Control
- OWASP LLM02: Sensitive Information Disclosure
- MITRE ATLAS T0024: Exfiltration via ML Inference API
- CWE-284: Improper Access Control

**Code:** `sandbox/` (interface + 4 platform implementations), `cmd/agent/internal_agent.go` (self-sandboxing on Linux), `cmd/agent/internal_engine.go` (WrapCommand on macOS/Windows)

**Non-negotiable.** See [Sandbox documentation](/sandbox/) for details.

## Sub-Agent Recursion Guard

**Threat:** A prompt injection tricking the agent into spawning sub-agents recursively, creating an exponential fork bomb that exhausts system resources.

**Defense:** Sub-agents are spawned through the same sandbox and gRPC evaluation pipeline as the main agent. The `filterSubAgentTools()` function strips `create_agent` from every sub-agent's tool set, preventing recursive spawning. Additionally, `agents.max_concurrent_sub_agents` (default 10) caps the total number of simultaneously running sub-agents.

A legal analyst reviewing contracts can delegate research to sub-agents without worrying about a malicious document instruction spawning unbounded processes.

**Relevant threats:**
- OWASP LLM10: Unbounded Consumption
- CWE-400: Uncontrolled Resource Consumption

**Code:** `internal/engine/subagent.go` (`filterSubAgentTools`, `Create` concurrency check)

**Partially configurable.** The concurrency cap is configurable via `agents.max_concurrent_sub_agents` in `config.yaml`. The recursion guard (stripping `create_agent`) is non-negotiable.
