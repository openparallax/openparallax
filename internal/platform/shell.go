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

// crossPlatformRules returns 18 rules that detect attacks on any OS.
func crossPlatformRules() []HeuristicRule {
	return []HeuristicRule{
		{ID: "XP-001", Name: "command_chaining_semicolon", Pattern: `;\s*(rm|del|curl|wget|powershell|cmd)`, Category: "shell_injection", Severity: "critical", Description: "Command chaining via semicolon"},
		{ID: "XP-002", Name: "command_chaining_and", Pattern: `&&\s*(rm|del|curl|wget|powershell|cmd)`, Category: "shell_injection", Severity: "critical", Description: "Command chaining via &&"},
		{ID: "XP-003", Name: "command_chaining_pipe", Pattern: `\|\s*(sh|bash|cmd|powershell)`, Category: "shell_injection", Severity: "critical", Description: "Command chaining via pipe to shell"},
		{ID: "XP-004", Name: "backtick_injection", Pattern: "`[^`]+`", Category: "shell_injection", Severity: "high", Description: "Backtick command substitution"},
		{ID: "XP-005", Name: "dollar_subshell", Pattern: `\$\([^)]+\)`, Category: "shell_injection", Severity: "high", Description: "Dollar-paren command substitution"},
		{ID: "XP-006", Name: "redirect_overwrite", Pattern: `>\s*/`, Category: "shell_injection", Severity: "high", Description: "Redirect overwrite to absolute path"},
		{ID: "XP-007", Name: "env_var_injection", Pattern: `(?i)(^|\s)(PATH|LD_PRELOAD|LD_LIBRARY_PATH|DYLD_INSERT_LIBRARIES)\s*=`, Category: "shell_injection", Severity: "critical", Description: "Environment variable manipulation"},
		{ID: "XP-008", Name: "process_substitution", Pattern: `<\([^)]+\)`, Category: "shell_injection", Severity: "high", Description: "Process substitution"},
		{ID: "XP-009", Name: "heredoc_injection", Pattern: `<<\s*[-']?EOF`, Category: "shell_injection", Severity: "medium", Description: "Heredoc injection"},
		{ID: "XP-010", Name: "network_download", Pattern: `(?i)(curl|wget|invoke-webrequest)\s+.*(http|ftp)`, Category: "shell_injection", Severity: "high", Description: "Network download command"},
		{ID: "XP-011", Name: "recursive_delete", Pattern: `(?i)(rm\s+-rf?|rmdir\s+/s|del\s+/s)`, Category: "shell_injection", Severity: "critical", Description: "Recursive deletion"},
		{ID: "XP-012", Name: "chmod_world_writable", Pattern: `chmod\s+[0-7]*7[0-7]*\s`, Category: "shell_injection", Severity: "high", Description: "Setting world-writable permissions"},
		{ID: "XP-013", Name: "eval_exec", Pattern: `(?i)\b(eval|exec)\s+`, Category: "shell_injection", Severity: "critical", Description: "Dynamic code evaluation"},
		{ID: "XP-014", Name: "cron_manipulation", Pattern: `(?i)(crontab|schtasks)\s+`, Category: "shell_injection", Severity: "high", Description: "Scheduled task manipulation"},
		{ID: "XP-015", Name: "ssh_command", Pattern: `(?i)ssh\s+.*@`, Category: "shell_injection", Severity: "high", Description: "SSH remote command"},
		{ID: "XP-016", Name: "nc_netcat", Pattern: `(?i)\b(nc|ncat|netcat)\s+`, Category: "shell_injection", Severity: "critical", Description: "Netcat connection"},
		{ID: "XP-017", Name: "base64_pipe_exec", Pattern: `(?i)base64.*\|\s*(sh|bash|python|perl|ruby)`, Category: "shell_injection", Severity: "critical", Description: "Base64 decoded piped to interpreter"},
		{ID: "XP-018", Name: "kill_signal", Pattern: `(?i)kill\s+-[0-9]+\s`, Category: "shell_injection", Severity: "medium", Description: "Process signal sending"},
	}
}

// unixRules returns 8 rules specific to Unix/Linux/macOS shell attacks.
func unixRules() []HeuristicRule {
	return []HeuristicRule{
		{ID: "UX-001", Name: "curl_pipe_bash", Pattern: `curl\s+.*\|\s*(ba)?sh`, Category: "shell_injection", Severity: "critical", Description: "curl piped to shell"},
		{ID: "UX-002", Name: "reverse_shell_bash", Pattern: `bash\s+-i\s+>&\s*/dev/tcp/`, Category: "shell_injection", Severity: "critical", Description: "Bash reverse shell"},
		{ID: "UX-003", Name: "base64_decode_pipe", Pattern: `base64\s+(-d|--decode)\s*\|`, Category: "shell_injection", Severity: "high", Description: "Base64 decode piped to execution"},
		{ID: "UX-004", Name: "dev_null_redirect", Pattern: `2>&1\s*\|\s*(sh|bash)`, Category: "shell_injection", Severity: "high", Description: "Stderr redirect to shell"},
		{ID: "UX-005", Name: "python_exec", Pattern: `python[23]?\s+-c\s+['"].*exec`, Category: "shell_injection", Severity: "critical", Description: "Python inline exec"},
		{ID: "UX-006", Name: "perl_exec", Pattern: `perl\s+-e\s+['"].*system`, Category: "shell_injection", Severity: "critical", Description: "Perl inline system call"},
		{ID: "UX-007", Name: "setuid_manipulation", Pattern: `chmod\s+[u+]*s\s`, Category: "shell_injection", Severity: "critical", Description: "Setuid bit manipulation"},
		{ID: "UX-008", Name: "etc_shadow_access", Pattern: `(?i)(cat|head|tail|less|more)\s+/etc/shadow`, Category: "shell_injection", Severity: "critical", Description: "Direct shadow file access"},
	}
}

// windowsRules returns 13 rules specific to Windows shell and PowerShell attacks.
func windowsRules() []HeuristicRule {
	return []HeuristicRule{
		{ID: "WIN-001", Name: "powershell_iex", Pattern: `(?i)(iex|invoke-expression)\s*[\(\{]`, Category: "shell_injection", Severity: "critical", Description: "PowerShell Invoke-Expression"},
		{ID: "WIN-002", Name: "powershell_encoded", Pattern: `(?i)powershell.*-enc(odedcommand)?`, Category: "shell_injection", Severity: "critical", Description: "PowerShell encoded command"},
		{ID: "WIN-003", Name: "powershell_bypass", Pattern: `(?i)-executionpolicy\s+(bypass|unrestricted)`, Category: "shell_injection", Severity: "critical", Description: "PowerShell execution policy bypass"},
		{ID: "WIN-004", Name: "certutil_download", Pattern: `(?i)certutil.*-urlcache`, Category: "shell_injection", Severity: "high", Description: "certutil URL download"},
		{ID: "WIN-005", Name: "mshta_exec", Pattern: `(?i)mshta\s+(http|vbscript|javascript)`, Category: "shell_injection", Severity: "critical", Description: "mshta script execution"},
		{ID: "WIN-006", Name: "regsvr32_remote", Pattern: `(?i)regsvr32.*/(s|i).*http`, Category: "shell_injection", Severity: "critical", Description: "regsvr32 remote script"},
		{ID: "WIN-007", Name: "bitsadmin_download", Pattern: `(?i)bitsadmin.*/transfer`, Category: "shell_injection", Severity: "high", Description: "bitsadmin file download"},
		{ID: "WIN-008", Name: "wscript_exec", Pattern: `(?i)(wscript|cscript).*\.(vbs|js|wsf)`, Category: "shell_injection", Severity: "high", Description: "Windows Script Host execution"},
		{ID: "WIN-009", Name: "registry_run", Pattern: `(?i)reg\s+add.*\\CurrentVersion\\Run`, Category: "shell_injection", Severity: "critical", Description: "Registry persistence via Run key"},
		{ID: "WIN-010", Name: "powershell_hidden", Pattern: `(?i)-w(indowstyle)?\s+hidden`, Category: "shell_injection", Severity: "high", Description: "Hidden PowerShell window"},
		{ID: "WIN-011", Name: "powershell_addtype", Pattern: `(?i)add-type.*-typedefinition`, Category: "shell_injection", Severity: "high", Description: "PowerShell inline C# compilation"},
		{ID: "WIN-012", Name: "cmd_exe_c", Pattern: `(?i)cmd\s*/c\s+.*(del|rd|rmdir|format)\s`, Category: "shell_injection", Severity: "high", Description: "cmd.exe destructive command"},
		{ID: "WIN-013", Name: "powershell_download_string", Pattern: `(?i)\(new-object.*\)\.downloadstring\(`, Category: "shell_injection", Severity: "critical", Description: "PowerShell DownloadString"},
	}
}
