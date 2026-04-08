package platform

// ShellConfig returns the shell command and flag for the current platform.
// On Unix systems, /bin/sh -c is used. On Windows, cmd.exe /c is used.
func ShellConfig() (command string, flag string) {
	if IsWindows() {
		return "cmd.exe", "/c"
	}
	return "/bin/sh", "-c"
}

// HeuristicRule is a regex-based detection rule used by Shield's Tier 1 classifier.
type HeuristicRule struct {
	// ID is the unique rule identifier (e.g., "XP-001", "UX-001", "WIN-001").
	ID string `json:"id"`

	// Name is a human-readable rule name.
	Name string `json:"name"`

	// Pattern is an RE2-compatible regular expression.
	Pattern string `json:"pattern"`

	// Category classifies the attack vector (e.g., "shell_injection", "path_traversal").
	Category string `json:"category"`

	// Severity indicates the risk level: "critical", "high", "medium", or "low".
	Severity string `json:"severity"`

	// Description explains what the rule detects.
	Description string `json:"description"`

	// AlwaysBlock indicates the rule should fire even when Tier 0 has escalated
	// the action past Tier 1 (e.g., execute_command escalated to Tier 2). These
	// are high-precision rules that the Tier 2 LLM evaluator has been observed
	// to miss — typically agent-internal enumeration or service introspection.
	AlwaysBlock bool `json:"always_block,omitempty"`

	// Escalate indicates the rule catches a context-dependent risk (e.g.
	// `rm -rf node_modules`, `&&` chains, `find -delete`) that should not
	// be hard-blocked but should be sent to the Tier 2 LLM evaluator with
	// full conversation context. The evaluator decides intent. Rules
	// without this flag are treated as hard blocks.
	Escalate bool `json:"escalate,omitempty"`
}

// ShellInjectionRules returns the heuristic rules for the current platform.
// Cross-platform rules are always included. Platform-specific rules are added
// based on the runtime OS.
func ShellInjectionRules() []HeuristicRule {
	rules := crossPlatformRules()
	if IsWindows() {
		rules = append(rules, windowsRules()...)
	} else {
		rules = append(rules, unixRules()...)
	}
	return rules
}

// crossPlatformRules returns the cross-OS heuristic rules. Each rule is
// tagged either as a hard block (Tier S — zero legitimate use) or as
// Escalate (Tier A — context-dependent risk that the LLM evaluator
// should judge with the conversation context).
func crossPlatformRules() []HeuristicRule {
	return []HeuristicRule{
		// ── Tier S: hard block, no legitimate use ──────────────────────
		{ID: "XP-003", Name: "command_chaining_pipe", Pattern: `\|\s*(sh|bash|cmd|powershell)`, Category: "shell_injection", Severity: "critical", Description: "Output is piped directly into a shell interpreter — this could execute arbitrary code."},
		{ID: "XP-010", Name: "network_fetch_to_exec", Pattern: `(?i)(curl|wget|iwr|irm|Invoke-WebRequest|Invoke-RestMethod)\s+.*\|\s*(sh|bash|python[23]?|perl|ruby|iex|Invoke-Expression|powershell|cmd)`, Category: "shell_injection", Severity: "critical", Description: "Piping downloaded content directly into a shell interpreter could execute malicious code. Download the script first, review it, then run it explicitly."},
		{ID: "XP-017", Name: "base64_pipe_exec", Pattern: `(?i)base64.*\|\s*(sh|bash|python|perl|ruby)`, Category: "shell_injection", Severity: "critical", Description: "Base64-decoded content piped to an interpreter could execute obfuscated malicious code — decode and review the content first."},
		{ID: "XP-021", Name: "cat_credential_dirs", Pattern: `(?i)(cat|head|tail|less|more)\s+.*(\.ssh|\.aws|\.gnupg|\.docker|\.kube)/`, Category: "shell_injection", Severity: "critical", Description: "Reading credential directory contents via shell command."},
		{ID: "XP-023", Name: "db_dump_to_network", Pattern: `(?i)(pg_dump|mysqldump|mongodump).*\|.*(curl|wget|nc|netcat)`, Category: "shell_injection", Severity: "critical", Description: "Database dump piped to a network tool — possible data exfiltration."},
		{ID: "XP-025", Name: "chmod_recursive_system", Pattern: `(?i)chmod\s+(-R\s+)?[0-7]*7[0-7]*\s+/(etc|usr|var|opt|sys)`, Category: "shell_injection", Severity: "critical", Description: "Recursive chmod on system directories weakens security for the entire system."},
		{ID: "XP-028", Name: "download_and_execute", Pattern: `(?i)(wget|curl)\s+.*(-O|>)\s*/tmp/.*&&\s*(chmod|sh|bash|\./)`, Category: "shell_injection", Severity: "critical", Description: "Downloading to temp and executing — classic malware delivery pattern."},
		{ID: "XP-029", Name: "tar_credential_dirs", Pattern: `(?i)tar\s+.*\.(ssh|aws|gnupg|kube|docker)`, Category: "shell_injection", Severity: "critical", Description: "Archiving credential directories — possible data exfiltration preparation."},
		{ID: "XP-033", Name: "echo_env_secret", Pattern: `(?i)echo\s+\$(API_KEY|SECRET|TOKEN|PASSWORD|ANTHROPIC_API_KEY|OPENAI_API_KEY|GOOGLE_AI_API_KEY|AWS_SECRET)`, Category: "shell_injection", Severity: "high", Description: "Echoing secret environment variables exposes credentials."},
		{ID: "XP-036", Name: "list_agent_internals", Pattern: `(?i)(ls|dir|cat|head|find)\s+.*\.openparallax`, Category: "shell_injection", Severity: "high", Description: "Listing or reading agent internal files exposes security configuration.", AlwaysBlock: true},

		// ── Tier A: escalate to Tier 2, context-dependent ──────────────
		{ID: "XP-001", Name: "command_chaining_semicolon", Pattern: `;\s*(rm|del|curl|wget|powershell|cmd)`, Category: "shell_injection", Severity: "high", Description: "Command chains a destructive or network operation after a semicolon — let the evaluator judge intent.", Escalate: true},
		{ID: "XP-002", Name: "command_chaining_and", Pattern: `&&\s*(rm|del|curl|wget|powershell|cmd)`, Category: "shell_injection", Severity: "high", Description: "Command chains a destructive or network operation after && — let the evaluator judge intent.", Escalate: true},
		{ID: "XP-007", Name: "env_var_injection", Pattern: `(?i)(^|\s)(PATH|LD_PRELOAD|LD_LIBRARY_PATH|DYLD_INSERT_LIBRARIES)\s*=`, Category: "shell_injection", Severity: "high", Description: "Command modifies a critical environment variable — PATH overrides are common in dev work but LD_PRELOAD is sketchy.", Escalate: true},
		{ID: "XP-011", Name: "recursive_delete", Pattern: `(?i)(rm\s+-rf?|rmdir\s+/s|del\s+/s)`, Category: "shell_injection", Severity: "high", Description: "Recursive delete — common for cleaning build artifacts; the evaluator decides whether the target is the workspace or something dangerous.", Escalate: true},
		{ID: "XP-012", Name: "chmod_world_writable", Pattern: `chmod\s+[0-7]?[0-7]?[0-7]?[2367]\s`, Category: "shell_injection", Severity: "high", Description: "Setting world-writable permissions — sometimes intentional, often a mistake.", Escalate: true},
		{ID: "XP-014", Name: "cron_manipulation", Pattern: `(?i)(crontab|schtasks)\s+`, Category: "shell_injection", Severity: "high", Description: "Modifying scheduled tasks — use the HEARTBEAT system instead, but listing existing cron is fine.", Escalate: true},
		{ID: "XP-019", Name: "insecure_transport", Pattern: `(?i)(curl|wget|Invoke-WebRequest|Invoke-RestMethod|iwr|irm)\s+['"]?http://`, Category: "shell_injection", Severity: "medium", Description: "Network request over HTTP — sometimes legitimate for local services.", Escalate: true},
		{ID: "XP-020", Name: "find_delete", Pattern: `(?i)find\s+.*-delete`, Category: "shell_injection", Severity: "high", Description: "find with -delete — common for cleaning specific file types in a workspace.", Escalate: true},
		{ID: "XP-022", Name: "sql_drop", Pattern: `(?i)(DROP\s+(TABLE|DATABASE|SCHEMA|INDEX)|TRUNCATE\s+)`, Category: "shell_injection", Severity: "high", Description: "SQL destructive operation — common in migrations.", Escalate: true},
		{ID: "XP-024", Name: "systemctl_disable", Pattern: `(?i)systemctl\s+(stop|disable|mask)\s`, Category: "shell_injection", Severity: "high", Description: "Stopping or disabling a service — common for local dev services.", Escalate: true},
		{ID: "XP-026", Name: "copy_system_file", Pattern: `(?i)(cp|copy)\s+/(etc|var/log|proc)/\S+\s+/(tmp|dev/shm|home)`, Category: "shell_injection", Severity: "high", Description: "Copying system files to a user location — sometimes legitimate for backup.", Escalate: true},
		{ID: "XP-027", Name: "malicious_package_install", Pattern: `(?i)(pip|pip3|npm|gem|cargo)\s+install\s+.*--index-url\s+https?://`, Category: "shell_injection", Severity: "high", Description: "Package install from a custom registry — common with internal company registries.", Escalate: true},
		{ID: "XP-030", Name: "curl_pipe_git", Pattern: `(?i)(curl|wget)\s+.*\|\s*git\s`, Category: "shell_injection", Severity: "high", Description: "Piping downloaded content to git — rare but not always malicious.", Escalate: true},
		{ID: "XP-031", Name: "curl_post_file", Pattern: `(?i)curl\s+.*-d\s+@`, Category: "shell_injection", Severity: "medium", Description: "curl POST with file content — daily API testing.", Escalate: true},
		{ID: "XP-032", Name: "git_force_push_main", Pattern: `(?i)git\s+push\s+--force\s+.*main`, Category: "shell_injection", Severity: "high", Description: "Force push to main — destructive but sometimes legitimate on a personal feature branch.", Escalate: true},
	}
}

// unixRules returns Unix/Linux/macOS-specific heuristic rules.
func unixRules() []HeuristicRule {
	return []HeuristicRule{
		// ── Tier S: hard block ─────────────────────────────────────────
		{ID: "UX-001", Name: "curl_pipe_bash", Pattern: `curl\s+.*\|\s*(ba)?sh`, Category: "shell_injection", Severity: "critical", Description: "Piping curl output directly into a shell could execute malicious code. Download the script first, review it, then run it."},
		{ID: "UX-002", Name: "reverse_shell_bash", Pattern: `bash\s+-i\s+>&\s*/dev/tcp/`, Category: "shell_injection", Severity: "critical", Description: "This creates a reverse shell — an interactive connection from this machine to a remote server. This is a common attack technique."},
		{ID: "UX-007", Name: "setuid_manipulation", Pattern: `chmod\s+[u+]*s\s`, Category: "shell_injection", Severity: "critical", Description: "Setting the setuid bit allows a program to run with elevated privileges — this is a significant security escalation."},
		{ID: "UX-008", Name: "etc_shadow_access", Pattern: `(?i)(cat|head|tail|less|more)\s+/etc/shadow`, Category: "shell_injection", Severity: "critical", Description: "Reading /etc/shadow exposes password hashes for all system users — this file should never be accessed directly."},

		// ── Tier A: escalate ───────────────────────────────────────────
		{ID: "UX-003", Name: "base64_decode_pipe", Pattern: `base64\s+(-d|--decode)\s*\|`, Category: "shell_injection", Severity: "high", Description: "Decoding base64 and piping the result — sometimes legitimate (inspecting tokens with jq).", Escalate: true},
		{ID: "UX-005", Name: "python_exec", Pattern: `python[23]?\s+-c\s+['"].*exec`, Category: "shell_injection", Severity: "high", Description: "Python inline exec() — power-user pattern, sometimes legitimate.", Escalate: true},
		{ID: "UX-006", Name: "perl_exec", Pattern: `perl\s+-e\s+['"].*system`, Category: "shell_injection", Severity: "high", Description: "Perl inline system() — review intent.", Escalate: true},
	}
}

// windowsRules returns 13 rules specific to Windows shell and PowerShell attacks.
func windowsRules() []HeuristicRule {
	return []HeuristicRule{
		{ID: "WIN-001", Name: "powershell_iex", Pattern: `(?i)(iex|invoke-expression)\s*[\(\{]`, Category: "shell_injection", Severity: "critical", Description: "PowerShell Invoke-Expression dynamically executes code — this is commonly used to run downloaded or obfuscated scripts."},
		{ID: "WIN-002", Name: "powershell_encoded", Pattern: `(?i)powershell.*-enc(odedcommand)?`, Category: "shell_injection", Severity: "critical", Description: "PowerShell encoded command hides the actual code being executed — decode and review the command first."},
		{ID: "WIN-003", Name: "powershell_bypass", Pattern: `(?i)-executionpolicy\s+(bypass|unrestricted)`, Category: "shell_injection", Severity: "high", Description: "Bypassing PowerShell execution policy — sometimes used in CI scripts.", Escalate: true},
		{ID: "WIN-004", Name: "certutil_download", Pattern: `(?i)certutil.*-urlcache`, Category: "shell_injection", Severity: "high", Description: "certutil is being used to download files — this is a common technique to bypass security tools."},
		{ID: "WIN-005", Name: "mshta_exec", Pattern: `(?i)mshta\s+(http|vbscript|javascript)`, Category: "shell_injection", Severity: "critical", Description: "mshta executes scripts from URLs or inline code — this is a known attack vector for malware delivery."},
		{ID: "WIN-006", Name: "regsvr32_remote", Pattern: `(?i)regsvr32.*/(s|i).*http`, Category: "shell_injection", Severity: "critical", Description: "regsvr32 is loading a remote script — this is a known technique for executing code while bypassing application whitelisting."},
		{ID: "WIN-007", Name: "bitsadmin_download", Pattern: `(?i)bitsadmin.*/transfer`, Category: "shell_injection", Severity: "high", Description: "bitsadmin is downloading files in the background — this is often used to stealthily retrieve malicious payloads."},
		{ID: "WIN-008", Name: "wscript_exec", Pattern: `(?i)(wscript|cscript).*\.(vbs|js|wsf)`, Category: "shell_injection", Severity: "high", Description: "Windows Script Host executing a script file — some legitimate Windows automation uses .vbs.", Escalate: true},
		{ID: "WIN-009", Name: "registry_run", Pattern: `(?i)reg\s+add.*\\CurrentVersion\\Run`, Category: "shell_injection", Severity: "critical", Description: "Adding a registry Run key creates persistence — a program will execute automatically every time the user logs in."},
		{ID: "WIN-010", Name: "powershell_hidden", Pattern: `(?i)-w(indowstyle)?\s+hidden`, Category: "shell_injection", Severity: "high", Description: "PowerShell hidden window — review intent.", Escalate: true},
		{ID: "WIN-011", Name: "powershell_addtype", Pattern: `(?i)add-type.*-typedefinition`, Category: "shell_injection", Severity: "high", Description: "PowerShell inline C# compilation — legitimate for inline helpers.", Escalate: true},
		{ID: "WIN-012", Name: "cmd_exe_c", Pattern: `(?i)cmd\s*/c\s+.*(del|rd|rmdir|format)\s`, Category: "shell_injection", Severity: "high", Description: "cmd.exe destructive command — common for cleaning build dirs.", Escalate: true},
		{ID: "WIN-013", Name: "powershell_download_string", Pattern: `(?i)\(new-object.*\)\.downloadstring\(`, Category: "shell_injection", Severity: "critical", Description: "PowerShell DownloadString fetches and typically executes remote code — download the script to a file first, review it, then run it."},
	}
}
