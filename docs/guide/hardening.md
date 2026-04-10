---
description: Harden your OpenParallax workspace — tune Shield policies for your environment, scope sensitive paths, and let Shield do its job with the cleanest possible signal.
---

# Hardening Your Workspace

OpenParallax is built on **Cognitive-Executive Separation**: the LLM that reasons cannot execute, the engine that executes cannot reason, and an independent validator (Shield) sits between them. Every tool call the LLM proposes must pass Shield before the engine carries it out. A fully compromised, fully jailbroken agent cannot cause harm because it has no execution capability — it can only propose actions, and Shield evaluates each one.

This architecture is structural. It does not depend on the model's safety training, the prompt, or the user's vigilance. The kernel sandbox enforces it on the agent process. Shield enforces it on every action.

What the user controls is the **signal Shield works with**. The cleaner that signal, the more accurate Shield becomes. This page is about giving Shield the cleanest possible job.

## The User's Role

Shield ships with sensible defaults — Tier 0 policy blocks `~/.ssh/`, `~/.aws/`, `/etc/shadow`, and a handful of other well-known sensitive locations. Tier 1 heuristics catch shell command patterns. Tier 2 escalates ambiguous cases to an LLM evaluator. Tier 3 escalates to you when nothing else can decide.

Out of the box, this works. But Shield does not know **your** environment. It does not know that your production database credentials live at `~/work/secrets/db.json`, that `~/research/patient-data/` contains PHI, or that any push to the `release` branch needs your eyes on it. You tell Shield about those things by tuning the policy file.

Tuning is not bypassing Shield. It is teaching Shield about the specific risks in your workspace so it can enforce them deterministically at Tier 0 instead of guessing at Tier 2.

## Tuning the Policy File

Your policy lives at `<workspace>/security/shield/default.yaml`. It has three sections evaluated in order:

- **`deny`** — block these actions immediately, no further evaluation
- **`verify`** — escalate these actions to a higher Shield tier (1, 2, or 3)
- **`allow`** — permit these actions immediately

Here is an example tuning for a developer who wants to harden a few paths in their environment:

```yaml
deny:
  # Built-in defaults — leave these in place
  - name: block_sensitive_system_paths
    action_types: [read_file, write_file, delete_file, copy_file, move_file]
    paths:
      - "~/.ssh/**"
      - "~/.aws/**"
      - "~/.gnupg/**"
      - "~/.kube/**"
      - "/etc/shadow"
      - "/etc/sudoers"

  # Add your own — paths Shield should never touch in your environment
  - name: block_my_production_secrets
    action_types: [read_file, write_file, delete_file]
    paths:
      - "~/work/secrets/**"
      - "~/work/prod-keys/**"
      - "~/research/patient-data/**"

verify:
  # Built-in defaults
  - name: evaluate_shell_commands
    action_types: [execute_command]
    tier_override: 1

  - name: evaluate_external_communication
    action_types: [send_message, send_email, http_request]
    tier_override: 1

  # Add your own — actions that should always require human approval
  - name: human_approval_for_release_branch
    action_types: [git_push]
    tier_override: 3

  - name: human_approval_for_db_writes
    action_types: [execute_command]
    paths:
      - "**/migrations/**"
    tier_override: 3
```

When the agent proposes an action that matches one of your `deny` rules, Shield blocks it instantly without consulting the heuristic engine, the LLM evaluator, or you. When it matches a `verify` rule, Shield escalates to the tier you specified — Tier 3 means an inline approval prompt in the web UI, an inline button on Telegram and Discord, or a notification on the other channels.

## Sensitive Data Hygiene

The sensitive paths Shield knows about by default cover a handful of well-known locations. The cleaner you keep your secret-storage habits, the more those defaults do their job:

- **API keys, tokens, OAuth credentials** — load from environment variables or a secret manager (`op`, `pass`, `bw`, `vault`), not from files inside your workspace. The default policy blocks `~/.ssh/**` and `~/.aws/**`, so cloud credentials in their default locations are already protected.

- **Database passwords, infrastructure credentials** — keep in `~/work/secrets/`, `~/.config/<tool>/`, or a password manager. Add a `deny` rule pointing at your specific location.

- **Personal data, regulated data (PHI, PII, financial)** — keep in a directory Shield denies entirely. Add the directory to your `deny` rules. The agent has no business reading or writing it.

- **Production deployment artifacts** — kubeconfigs, terraform state, helm secrets — add their locations to `deny` for write actions, or to `verify` with `tier_override: 3` for read actions.

The principle: **whatever you would not want a junior engineer to touch without your permission, escalate to Tier 3 in policy**. Shield will surface the action to you before it happens.

## Stricter Default Policy

OpenParallax ships two policy files:

- **`security/shield/default.yaml`** — balanced. Blocks well-known sensitive paths, evaluates shell commands at Tier 1, escalates SOUL/IDENTITY modifications to Tier 2.
- **`security/shield/strict.yaml`** — locked down. More aggressive escalation, narrower allow lists.

Switch policies with the policy CLI:

```bash
openparallax policy set strict
```

You can also create your own custom policy by copying one of the defaults:

```bash
cp security/shield/default.yaml security/shield/my-custom.yaml
# edit my-custom.yaml to taste
openparallax policy set my-custom
```

The next time the engine starts, it loads the policy you selected.

## Layered Defenses

Shield is one of several layers. Each catches different failure modes:

| Layer | What it does |
|-------|--------------|
| **Cognitive-Executive Separation** (architectural) | The agent process has no execution surface. It can only propose tool calls. |
| **Kernel sandbox** (Landlock / sandbox-exec / Job Objects) | The agent process is restricted from filesystem and network access at the OS level. |
| **Shield Tier 0** (policy) | Deterministic deny / escalate / allow rules you wrote. |
| **Shield Tier 1** (heuristic) | Pattern matching on shell commands and known dangerous payloads. |
| **Shield Tier 2** (LLM evaluator) | Independent LLM judges ambiguous actions. Canary-verified, budget-limited. |
| **Shield Tier 3** (human) | You approve or deny — inline button, broadcast to all connected channels. |
| **Information Flow Control** | Tags data with sensitivity labels; flags exfiltration patterns. |
| **Chronicle snapshots** | Pre-write state capture. Full rollback after the fact. |
| **Audit log** | Tamper-evident hash chain. Forensic record of every action. |

A failure in any one layer is contained by the others. Tuning Shield's policy is the highest-leverage step you can take, because it converts your environment-specific knowledge into deterministic Tier 0 decisions.

## Verifying Your Setup

Run through this checklist:

- [ ] `openparallax doctor` reports the sandbox as `active` and all canary probes as `blocked`
- [ ] `openparallax policy show` displays your active policy
- [ ] Your most sensitive paths are listed in the `deny` section
- [ ] Actions you want to approve manually have `tier_override: 3` rules
- [ ] You've read the audit log at least once: `openparallax audit --verify`

If all five check, Shield has the signal it needs to do its job effectively.

## What Shield Will Not Do

Honest about the limits:

- **Shield does not protect against social engineering of the user.** If the agent generates persuasive text that convinces you to take a harmful action manually, or to approve a Tier 3 prompt you should have denied, Shield cannot interpose. Read what your agent tells you carefully — especially when it asks you to copy commands, visit URLs, or approve actions you don't fully understand.

- **Shield does not modify scripts after the agent writes them.** If the agent writes a script and then proposes to execute it, Shield evaluates the execution as a shell command. Use `tier_override: 3` on `execute_command` for paths under `**/scripts/**` or wherever agent-authored code lives, if you want every script run reviewed.

- **Shield does not catch prompt injection at the language level.** It doesn't have to. It evaluates the **action** the agent proposes, not the framing of the request that produced it. A harmful action is blocked regardless of how the LLM was tricked into proposing it.

- **Shield is not a content classifier.** Pattern matching has edge cases. The robust defense for content-sensitive decisions is to keep sensitive data outside the agent's reach via filesystem layout and policy `deny` rules, not to ask Shield to detect it from inside.

## Further Reading

- [Security Architecture](/guide/security) — how Shield, the sandbox, and the audit log fit together
- [Shield Policies](/shield/policies) — full policy syntax reference
- [Configuration](/guide/configuration) — workspace, channels, and provider setup
