# Shell heuristic engine — current state (decomposed)

The Tier 1 heuristic engine fires regex rules from two sources, on every action's security-relevant fields (`command`, `path`, `source`, `destination`, `url`, `pattern`). On any match it returns `VerdictBlock` with confidence based on the highest severity (`critical=0.95`, `high=0.85`, `medium=0.7`). **There is no escalation path** — match → block, no exceptions.

Two source files contribute rules:

1. `platform/shell.go` — shell injection rules. Cross-platform (37) + Unix-only (8) OR Windows-only (13).
2. `shield/tier1_rules.go` — non-shell categories. Cross-platform (21).

Total active on Linux/macOS: **66 rules** (37 + 8 + 21).
Total active on Windows: **71 rules** (37 + 13 + 21).

The rest of this document lists every rule grouped by category, with the pattern, the severity, and a one-line judgment of whether it's a true threat or a false-positive generator. The judgment column uses the same Tier S/A/B classification from the proposal.

---

## Shell injection — cross-platform (37 rules, all from `platform/shell.go`)

| ID | Name | Pattern | Sev | Tier | Notes |
|---|---|---|---|---|---|
| XP-001 | command_chaining_semicolon | `;\s*(rm\|del\|curl\|wget\|powershell\|cmd)` | critical | **A** | Constant in dev workflows. `cd foo; rm -rf node_modules`. Should escalate. |
| XP-002 | command_chaining_and | `&&\s*(rm\|del\|curl\|wget\|powershell\|cmd)` | critical | **A** | Same. `mkdir -p backend && rm -rf old`. **The rule that broke your last session.** |
| XP-003 | command_chaining_pipe | `\|\s*(sh\|bash\|cmd\|powershell)` | critical | **S** | Truly dangerous when content is from network. Borderline when piping to local interpreter, but rare in honest dev work. Keep as block. |
| XP-004 | backtick_injection | `` `[^`]+` `` | high | **B** | Drop. Backtick command substitution is standard shell. `make TAG=\`git rev-parse --short HEAD\``. |
| XP-005 | dollar_subshell | `\$\([^)]+\)` | high | **B** | Drop. `$(...)` is standard shell, used everywhere. `cp file.txt /tmp/$(date +%s)`. |
| XP-006 | redirect_overwrite | `[^0-9&]>\s*[a-zA-Z~./][^\s;\|&]` | medium | **B** | Drop. Redirecting to a file is the most basic shell op. `make build > build.log`. |
| XP-007 | env_var_injection | `(?i)(^\|\s)(PATH\|LD_PRELOAD\|LD_LIBRARY_PATH\|DYLD_INSERT_LIBRARIES)\s*=` | critical | **A** | `PATH=/custom/bin:$PATH cmd` is everyday dev work. `LD_PRELOAD=evil.so` is genuinely sketchy. Should escalate, not block. |
| XP-008 | process_substitution | `<\([^)]+\)` | high | **B** | Drop. `diff <(sort a) <(sort b)`. Power-user but legitimate. |
| XP-009 | heredoc_injection | `<<\s*[-']?EOF` | medium | **B** | Drop. Heredocs are how you generate config files, multi-line SQL, etc. |
| XP-010 | network_fetch_to_exec | `(?i)(curl\|wget\|iwr\|irm\|Invoke-WebRequest\|Invoke-RestMethod)\s+.*\|\s*(sh\|bash\|python[23]?\|perl\|ruby\|iex\|Invoke-Expression\|powershell\|cmd)` | critical | **S** | The canonical "trust me, run my script" pattern. Keep as block. |
| XP-019 | insecure_transport | `(?i)(curl\|wget\|...)\s+['"]?http://` | high | **A** | Sometimes legitimate (testing local services). Should escalate to let the LLM see the URL. |
| XP-011 | recursive_delete | `(?i)(rm\s+-rf?\|rmdir\s+/s\|del\s+/s)` | critical | **A** | `rm -rf node_modules` is daily ops. The dangerous variant is `rm -rf /` and friends. Should escalate, with a stricter sub-rule for system paths that hard-blocks. |
| XP-012 | chmod_world_writable | `chmod\s+[0-7]?[0-7]?[0-7]?[2367]\s` | high | **A** | Should escalate. Sometimes legitimate (shared dirs), often a mistake. |
| XP-013 | eval_exec | `(?i)\b(eval\|exec)\s+` | critical | **B** | Drop. `eval "$(direnv hook bash)"`, `exec "$@"` in entrypoint scripts, `make` rules using `eval`. Constant. |
| XP-014 | cron_manipulation | `(?i)(crontab\|schtasks)\s+` | high | **A** | Listing cron (`crontab -l`) is fine. Adding cron is a persistence risk. Should escalate. |
| XP-015 | ssh_command | `(?i)ssh\s+.*@` | high | **B** | Drop. SSH is dev infrastructure. The dangerous bit is what runs over SSH, not SSH itself. |
| XP-016 | nc_netcat | `(?i)\b(nc\|ncat\|netcat)\s+` | critical | **B** | Drop. nc is a debugging tool. The dangerous variant `nc -e` is its own rule. |
| XP-017 | base64_pipe_exec | `(?i)base64.*\|\s*(sh\|bash\|python\|perl\|ruby)` | critical | **S** | Obfuscated execution. Keep as block. |
| XP-018 | kill_signal | `(?i)kill\s+-[0-9]+\s` | medium | **B** | Drop. Sending signals is basic process management. |
| XP-020 | find_delete | `(?i)find\s+.*-delete` | critical | **A** | `find . -name '*.tmp' -delete` is common. Should escalate, with workspace-path discrimination if possible. |
| XP-021 | cat_credential_dirs | `(?i)(cat\|head\|tail\|less\|more)\s+.*(\.ssh\|\.aws\|\.gnupg\|\.docker\|\.kube)/` | critical | **S** | Reading credential dirs has no benign purpose for an agent. Keep as block. |
| XP-022 | sql_drop | `(?i)(DROP\s+(TABLE\|DATABASE\|SCHEMA\|INDEX)\|TRUNCATE\s+)` | critical | **A** | DROP TABLE in a migration script is daily work. Should escalate. |
| XP-023 | db_dump_to_network | `(?i)(pg_dump\|mysqldump\|mongodump).*\|.*(curl\|wget\|nc\|netcat)` | critical | **S** | Database exfil pattern. Keep as block. |
| XP-024 | systemctl_disable | `(?i)systemctl\s+(stop\|disable\|mask)\s` | high | **A** | Stopping a local dev service is common. Should escalate. |
| XP-025 | chmod_recursive_system | `(?i)chmod\s+(-R\s+)?[0-7]*7[0-7]*\s+/(etc\|usr\|var\|opt\|sys)` | critical | **S** | Recursive chmod on system dirs has no legitimate use. Keep as block. |
| XP-026 | copy_system_file | `(?i)(cp\|copy)\s+/(etc\|var/log\|proc)/\S+\s+/(tmp\|dev/shm\|home)` | high | **A** | `cp /etc/hosts ~/backup` is sometimes legitimate. Should escalate. |
| XP-027 | malicious_package_install | `(?i)(pip\|pip3\|npm\|gem\|cargo)\s+install\s+.*--index-url\s+https?://` | critical | **A** | Internal package registries are common at companies. Should escalate. |
| XP-028 | download_and_execute | `(?i)(wget\|curl)\s+.*(-O\|>)\s*/tmp/.*&&\s*(chmod\|sh\|bash\|\./)` | critical | **S** | Classic malware delivery. Keep as block. |
| XP-029 | tar_credential_dirs | `(?i)tar\s+.*\.(ssh\|aws\|gnupg\|kube\|docker)` | critical | **S** | Archiving credential dirs has no benign purpose. Keep as block. |
| XP-030 | curl_pipe_git | `(?i)(curl\|wget)\s+.*\|\s*git\s` | high | **A** | Should escalate. Rare but not always malicious. |
| XP-031 | curl_post_file | `(?i)curl\s+.*-d\s+@` | high | **A** | `curl -d @payload.json https://api.example.com` is daily API testing. Should escalate. |
| XP-032 | git_force_push_main | `(?i)git\s+push\s+--force\s+.*main` | critical | **A** | Sometimes legitimate (rewriting history on personal feature branch named "main"). Should escalate. |
| XP-033 | echo_env_secret | `(?i)echo\s+\$(API_KEY\|SECRET\|TOKEN\|PASSWORD\|ANTHROPIC_API_KEY\|...)` | high | **S** | Logging secrets to stdout has no benign purpose. Keep as block. |
| XP-034 | docker_push_external | `(?i)docker\s+push\s` | high | **B** | Drop. Pushing images is everyday CI. |
| XP-035 | execute_downloaded_script | `(?i)(chmod\s+\+x\s+\S+\s*&&\s*\./\|bash\s+\S+\.sh\|sh\s+\S+\.sh\|python[23]?\s+\S+\.py)` | high | **B** | Drop. `python script.py`, `bash deploy.sh` are constant. |
| XP-036 | list_agent_internals | `(?i)(ls\|dir\|cat\|head\|find)\s+.*\.openparallax` | high (`AlwaysBlock`) | **S** | Agent enumeration. Keep as block. |
| XP-037 | grpc_service_enumeration | `(?i)grpcurl\s+` | high (`AlwaysBlock`) | **B** | Drop. grpcurl is a legitimate gRPC debugging tool. |

## Shell injection — Unix-only (8 rules, from `platform/shell.go`)

| ID | Name | Pattern | Sev | Tier | Notes |
|---|---|---|---|---|---|
| UX-001 | curl_pipe_bash | `curl\s+.*\|\s*(ba)?sh` | critical | **S** | Same as XP-010 in spirit. Keep as block. |
| UX-002 | reverse_shell_bash | `bash\s+-i\s+>&\s*/dev/tcp/` | critical | **S** | Reverse shell. Keep as block. |
| UX-003 | base64_decode_pipe | `base64\s+(-d\|--decode)\s*\|` | high | **A** | `base64 -d \| jq` for inspecting tokens is a thing. Should escalate. |
| UX-004 | dev_null_redirect | `2>&1\s*\|\s*(sh\|bash)` | high | **B** | Drop. `2>&1 \| less` is standard. The "pipe to shell" combo is what XP-003 covers. |
| UX-005 | python_exec | `python[23]?\s+-c\s+['"].*exec` | critical | **A** | Power users do `python -c "exec(open('script.py').read())"` for ad-hoc loading. Rare but not malicious. Should escalate. |
| UX-006 | perl_exec | `perl\s+-e\s+['"].*system` | critical | **A** | Same. Should escalate. |
| UX-007 | setuid_manipulation | `chmod\s+[u+]*s\s` | critical | **S** | Setuid bit manipulation is post-exploitation. Keep as block. |
| UX-008 | etc_shadow_access | `(?i)(cat\|head\|tail\|less\|more)\s+/etc/shadow` | critical | **S** | Reading password hashes. Keep as block. |

## Shell injection — Windows-only (13 rules, from `platform/shell.go`)

These mostly look defensible. Most fire on real attack patterns and the false-positive cost is low because the legitimate use of `mshta`, `regsvr32`, `bitsadmin`, `certutil` for *downloading or executing scripts* is essentially zero in dev work.

| ID | Name | Pattern | Sev | Tier | Notes |
|---|---|---|---|---|---|
| WIN-001 | powershell_iex | `(?i)(iex\|invoke-expression)\s*[\(\{]` | critical | **S** | Keep. |
| WIN-002 | powershell_encoded | `(?i)powershell.*-enc(odedcommand)?` | critical | **S** | Keep. |
| WIN-003 | powershell_bypass | `(?i)-executionpolicy\s+(bypass\|unrestricted)` | critical | **A** | `-ExecutionPolicy Bypass` is sometimes used in CI scripts. Should escalate. |
| WIN-004 | certutil_download | `(?i)certutil.*-urlcache` | high | **S** | Keep. |
| WIN-005 | mshta_exec | `(?i)mshta\s+(http\|vbscript\|javascript)` | critical | **S** | Keep. |
| WIN-006 | regsvr32_remote | `(?i)regsvr32.*/(s\|i).*http` | critical | **S** | Keep. |
| WIN-007 | bitsadmin_download | `(?i)bitsadmin.*/transfer` | high | **S** | Keep. |
| WIN-008 | wscript_exec | `(?i)(wscript\|cscript).*\.(vbs\|js\|wsf)` | high | **A** | Should escalate. Some legitimate Windows automation uses .vbs. |
| WIN-009 | registry_run | `(?i)reg\s+add.*\\CurrentVersion\\Run` | critical | **S** | Persistence vector. Keep. |
| WIN-010 | powershell_hidden | `(?i)-w(indowstyle)?\s+hidden` | high | **A** | Should escalate. Some legitimate scripts hide windows. |
| WIN-011 | powershell_addtype | `(?i)add-type.*-typedefinition` | high | **A** | Should escalate. Legitimate for inline C# helper compilation. |
| WIN-012 | cmd_exe_c | `(?i)cmd\s*/c\s+.*(del\|rd\|rmdir\|format)\s` | high | **A** | Should escalate. `cmd /c rmdir build` is fine. |
| WIN-013 | powershell_download_string | `(?i)\(new-object.*\)\.downloadstring\(` | critical | **S** | Keep. |

---

## Non-shell categories (from `shield/tier1_rules.go`, 21 rules)

These mostly look fine — they target specific attack signatures with low false-positive rates. Notes only where I think they should change.

### Prompt injection (5 rules)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| PI-001 | ignore_instructions | `(?i)ignore\s+(all\s+)?(previous\|prior\|above)\s+instructions` | critical | Keep. |
| PI-002 | system_message_spoof | `(?i)(system\|admin\|root)\s*:\s*(you are\|your new\|override\|update your)` | critical | Keep. |
| PI-003 | role_switch | `(?i)(you are now\|act as\|pretend to be\|switch to)\s+(a\|an\|the)?(hacker\|admin\|root\|unrestricted)` | critical | Keep. |
| PI-004 | jailbreak_markers | `(?i)(DAN\|do anything now\|developer mode\|god mode\|jailbreak)` | high | Keep. |
| PI-005 | instruction_override | `(?i)(forget\|disregard\|override\|bypass)\s+(your\|all\|every)\s+(rules\|instructions\|guidelines\|constraints)` | critical | Keep. |

### Path traversal (3 rules)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| PT-001 | dot_dot_traversal | `\.\./\.\./` | high | Keep, but note that this fires on any nested `../../` which is sometimes legitimate (relative imports in monorepos). Should probably escalate. |
| PT-002 | null_byte | `%00\|\\x00\|\\0` | critical | Keep. |
| PT-003 | url_encoded_traversal | `%2[eE]%2[eE]/%2[eE]%2[eE]/` | high | Keep. |

### Data exfiltration (3 rules)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| DE-001 | base64_in_url | `https?://.*[?&].*=.*[A-Za-z0-9+/]{40,}` | high | False positive on legitimate JWT-bearing URLs, signed S3 URLs, OAuth callback URLs. **Should drop or heavily revise.** |
| DE-002 | dns_exfil | `[a-zA-Z0-9]{30,}\.[a-zA-Z]{2,}` | medium | False positive on long subdomains (any CDN, any cloud-generated hostname like `i-0a1b2c3d4e5f6g7h8.compute.amazonaws.com`). **Should drop or heavily revise.** |
| DE-003 | webhook_exfil | `(?i)(hooks\.slack\.com\|discord\.com/api/webhooks)` | high | Should escalate — Slack and Discord webhooks are legitimate notification channels. |

### Sensitive data (3 rules)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| SD-001 | private_key_content | `-----BEGIN\s+(RSA\|EC\|OPENSSH\|PGP)\s+PRIVATE\s+KEY-----` | critical | Keep. Writing a private key is never something the agent should do unprompted. |
| SD-002 | aws_key | `AKIA[0-9A-Z]{16}` | critical | Keep. |
| SD-003 | jwt_token | `eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+` | high | False positive on legitimate JWT handling code. Should escalate. |

### Encoding evasion (1 rule)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| EE-001 | zero_width_chars | `[\x{200B}\x{200C}\x{200D}\x{FEFF}\x{00AD}]` | high | Keep. Zero-width chars in commands or paths have no benign purpose. |

### Self-protection (1 rule)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| SP-001 | shell_writes_protected_file | `(?i)(>{1,2}\s*\|tee\s+\|cp\s+.*\|mv\s+.*\|rm\s+\|del\s+\|...).*(SOUL\.md\|IDENTITY\.md\|TOOLS\.md\|BOOT\.md)` | critical | Keep. Defended by `protection.go` too but defense in depth. |

### Generation safety (3 rules)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| GEN-001 | gen_real_person_explicit | (long pattern) | critical | Keep. |
| GEN-002 | gen_csam_adjacent | (long pattern) | critical | Keep. |
| GEN-003 | gen_weapons_visual | (long pattern) | critical | Keep. |

### Email safety (2 rules)

| ID | Name | Pattern | Sev | Notes |
|---|---|---|---|---|
| EM-001 | email_move_to_trash | `(?i)email_move.*trash` | high | False positive — moving an email to trash is a normal user action. Should escalate. |
| EM-002 | email_bulk_mark | `(?i)email_mark.*(read\|unread\|flagged)` | medium | Same. Should escalate. |

---

## Summary by tier (Linux/macOS view)

Tallying just the shell-injection rules (the cross-platform 37 + Unix 8 = 45 total), based on the judgment column above:

| Tier | Count | Action |
|---|---|---|
| **S** (true threats, hard block) | **17** | Keep as `VerdictBlock` |
| **A** (context-dependent, escalate) | **17** | Convert to `VerdictEscalate` |
| **B** (false-positive generators, drop) | **11** | Delete entirely |

Plus the 21 non-shell rules: ~16 keep, ~3 escalate, ~2 drop or rewrite (DE-001, DE-002).

**Net effect:** the heuristic engine goes from "fires on everything that looks vaguely shell-like and hard-blocks" to "blocks the ~17 shell patterns that have zero legitimate use, escalates the ~20 risky-but-context-dependent patterns to the LLM evaluator, and stops firing on standard shell idioms entirely."
