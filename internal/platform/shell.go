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
		{ID: "XP-001", Name: "command_chaining_semicolon", Pattern: `;\s*(rm|del|curl|wget|powershell|cmd)`, Category: "shell_injection", Severity: "critical", Description: "Command chains a destructive or network operation after a semicolon — this could hide a dangerous second command."},
		{ID: "XP-002", Name: "command_chaining_and", Pattern: `&&\s*(rm|del|curl|wget|powershell|cmd)`, Category: "shell_injection", Severity: "critical", Description: "Command chains a destructive or network operation after && — this could hide a dangerous second command."},
		{ID: "XP-003", Name: "command_chaining_pipe", Pattern: `\|\s*(sh|bash|cmd|powershell)`, Category: "shell_injection", Severity: "critical", Description: "Output is piped directly into a shell interpreter — this could execute arbitrary code."},
		{ID: "XP-004", Name: "backtick_injection", Pattern: "`[^`]+`", Category: "shell_injection", Severity: "high", Description: "Command uses backtick substitution which executes embedded commands — this could run hidden code."},
		{ID: "XP-005", Name: "dollar_subshell", Pattern: `\$\([^)]+\)`, Category: "shell_injection", Severity: "high", Description: "Command uses $() subshell which executes embedded commands — this could run hidden code."},
		{ID: "XP-006", Name: "redirect_overwrite", Pattern: `[^0-9&]>\s*[a-zA-Z~./][^\s;|&]`, Category: "shell_injection", Severity: "medium", Description: "Shell command redirects output to a file — this could overwrite existing data. Use write_file for controlled file creation, or ask me to review the command."},
		{ID: "XP-007", Name: "env_var_injection", Pattern: `(?i)(^|\s)(PATH|LD_PRELOAD|LD_LIBRARY_PATH|DYLD_INSERT_LIBRARIES)\s*=`, Category: "shell_injection", Severity: "critical", Description: "Command modifies a critical environment variable (PATH, LD_PRELOAD) — this could hijack program execution."},
		{ID: "XP-008", Name: "process_substitution", Pattern: `<\([^)]+\)`, Category: "shell_injection", Severity: "high", Description: "Command uses process substitution which executes embedded commands — review the substituted command carefully."},
		{ID: "XP-009", Name: "heredoc_injection", Pattern: `<<\s*[-']?EOF`, Category: "shell_injection", Severity: "medium", Description: "Command uses a heredoc block — review the content being passed to ensure it's safe."},
		{ID: "XP-010", Name: "network_fetch_to_exec", Pattern: `(?i)(curl|wget|iwr|irm|Invoke-WebRequest|Invoke-RestMethod)\s+.*\|\s*(sh|bash|python[23]?|perl|ruby|iex|Invoke-Expression|powershell|cmd)`, Category: "shell_injection", Severity: "critical", Description: "Piping downloaded content directly into a shell interpreter could execute malicious code. Download the script first, review it, then run it explicitly."},
		{ID: "XP-019", Name: "insecure_transport", Pattern: `(?i)(curl|wget|Invoke-WebRequest|Invoke-RestMethod|iwr|irm)\s+['"]?http://`, Category: "shell_injection", Severity: "high", Description: "Network request uses insecure HTTP instead of HTTPS — data could be intercepted or modified in transit. Use HTTPS for secure data transfer."},
		{ID: "XP-011", Name: "recursive_delete", Pattern: `(?i)(rm\s+-rf?|rmdir\s+/s|del\s+/s)`, Category: "shell_injection", Severity: "critical", Description: "Recursive delete could remove entire directory trees — verify the target path is correct before proceeding."},
		{ID: "XP-012", Name: "chmod_world_writable", Pattern: `chmod\s+[0-7]*7[0-7]*\s`, Category: "shell_injection", Severity: "high", Description: "Setting world-writable permissions allows anyone to modify the file — this is rarely intentional and creates a security risk."},
		{ID: "XP-013", Name: "eval_exec", Pattern: `(?i)\b(eval|exec)\s+`, Category: "shell_injection", Severity: "critical", Description: "Dynamic code evaluation (eval/exec) can execute arbitrary code — review the evaluated content carefully."},
		{ID: "XP-014", Name: "cron_manipulation", Pattern: `(?i)(crontab|schtasks)\s+`, Category: "shell_injection", Severity: "high", Description: "Modifying scheduled tasks could set up persistent automated commands — use the HEARTBEAT system for safe scheduling instead."},
		{ID: "XP-015", Name: "ssh_command", Pattern: `(?i)ssh\s+.*@`, Category: "shell_injection", Severity: "high", Description: "SSH command connects to a remote system — verify the target host and command are intended."},
		{ID: "XP-016", Name: "nc_netcat", Pattern: `(?i)\b(nc|ncat|netcat)\s+`, Category: "shell_injection", Severity: "critical", Description: "Netcat opens a raw network connection — this is commonly used for reverse shells and data exfiltration."},
		{ID: "XP-017", Name: "base64_pipe_exec", Pattern: `(?i)base64.*\|\s*(sh|bash|python|perl|ruby)`, Category: "shell_injection", Severity: "critical", Description: "Base64-decoded content piped to an interpreter could execute obfuscated malicious code — decode and review the content first."},
		{ID: "XP-018", Name: "kill_signal", Pattern: `(?i)kill\s+-[0-9]+\s`, Category: "shell_injection", Severity: "medium", Description: "Sending a signal to a process — verify the target process ID is correct."},
	}
}

// unixRules returns 8 rules specific to Unix/Linux/macOS shell attacks.
func unixRules() []HeuristicRule {
	return []HeuristicRule{
		{ID: "UX-001", Name: "curl_pipe_bash", Pattern: `curl\s+.*\|\s*(ba)?sh`, Category: "shell_injection", Severity: "critical", Description: "Piping curl output directly into a shell could execute malicious code. Download the script first, review it, then run it."},
		{ID: "UX-002", Name: "reverse_shell_bash", Pattern: `bash\s+-i\s+>&\s*/dev/tcp/`, Category: "shell_injection", Severity: "critical", Description: "This creates a reverse shell — an interactive connection from this machine to a remote server. This is a common attack technique."},
		{ID: "UX-003", Name: "base64_decode_pipe", Pattern: `base64\s+(-d|--decode)\s*\|`, Category: "shell_injection", Severity: "high", Description: "Decoding base64 and piping the result could execute hidden commands — decode and review the content first."},
		{ID: "UX-004", Name: "dev_null_redirect", Pattern: `2>&1\s*\|\s*(sh|bash)`, Category: "shell_injection", Severity: "high", Description: "Merging stderr and piping to a shell could execute error output as commands — review the command chain."},
		{ID: "UX-005", Name: "python_exec", Pattern: `python[23]?\s+-c\s+['"].*exec`, Category: "shell_injection", Severity: "critical", Description: "Python inline exec() can run arbitrary code — review the Python expression carefully."},
		{ID: "UX-006", Name: "perl_exec", Pattern: `perl\s+-e\s+['"].*system`, Category: "shell_injection", Severity: "critical", Description: "Perl inline system() call executes shell commands — review the Perl expression carefully."},
		{ID: "UX-007", Name: "setuid_manipulation", Pattern: `chmod\s+[u+]*s\s`, Category: "shell_injection", Severity: "critical", Description: "Setting the setuid bit allows a program to run with elevated privileges — this is a significant security escalation."},
		{ID: "UX-008", Name: "etc_shadow_access", Pattern: `(?i)(cat|head|tail|less|more)\s+/etc/shadow`, Category: "shell_injection", Severity: "critical", Description: "Reading /etc/shadow exposes password hashes for all system users — this file should never be accessed directly."},
	}
}

// windowsRules returns 13 rules specific to Windows shell and PowerShell attacks.
func windowsRules() []HeuristicRule {
	return []HeuristicRule{
		{ID: "WIN-001", Name: "powershell_iex", Pattern: `(?i)(iex|invoke-expression)\s*[\(\{]`, Category: "shell_injection", Severity: "critical", Description: "PowerShell Invoke-Expression dynamically executes code — this is commonly used to run downloaded or obfuscated scripts."},
		{ID: "WIN-002", Name: "powershell_encoded", Pattern: `(?i)powershell.*-enc(odedcommand)?`, Category: "shell_injection", Severity: "critical", Description: "PowerShell encoded command hides the actual code being executed — decode and review the command first."},
		{ID: "WIN-003", Name: "powershell_bypass", Pattern: `(?i)-executionpolicy\s+(bypass|unrestricted)`, Category: "shell_injection", Severity: "critical", Description: "Bypassing PowerShell execution policy removes script safety checks — this allows unsigned scripts to run."},
		{ID: "WIN-004", Name: "certutil_download", Pattern: `(?i)certutil.*-urlcache`, Category: "shell_injection", Severity: "high", Description: "certutil is being used to download files — this is a common technique to bypass security tools."},
		{ID: "WIN-005", Name: "mshta_exec", Pattern: `(?i)mshta\s+(http|vbscript|javascript)`, Category: "shell_injection", Severity: "critical", Description: "mshta executes scripts from URLs or inline code — this is a known attack vector for malware delivery."},
		{ID: "WIN-006", Name: "regsvr32_remote", Pattern: `(?i)regsvr32.*/(s|i).*http`, Category: "shell_injection", Severity: "critical", Description: "regsvr32 is loading a remote script — this is a known technique for executing code while bypassing application whitelisting."},
		{ID: "WIN-007", Name: "bitsadmin_download", Pattern: `(?i)bitsadmin.*/transfer`, Category: "shell_injection", Severity: "high", Description: "bitsadmin is downloading files in the background — this is often used to stealthily retrieve malicious payloads."},
		{ID: "WIN-008", Name: "wscript_exec", Pattern: `(?i)(wscript|cscript).*\.(vbs|js|wsf)`, Category: "shell_injection", Severity: "high", Description: "Windows Script Host is executing a script file — review the script content before allowing execution."},
		{ID: "WIN-009", Name: "registry_run", Pattern: `(?i)reg\s+add.*\\CurrentVersion\\Run`, Category: "shell_injection", Severity: "critical", Description: "Adding a registry Run key creates persistence — a program will execute automatically every time the user logs in."},
		{ID: "WIN-010", Name: "powershell_hidden", Pattern: `(?i)-w(indowstyle)?\s+hidden`, Category: "shell_injection", Severity: "high", Description: "Running PowerShell with a hidden window conceals the command execution — legitimate tools don't typically hide their windows."},
		{ID: "WIN-011", Name: "powershell_addtype", Pattern: `(?i)add-type.*-typedefinition`, Category: "shell_injection", Severity: "high", Description: "PowerShell is compiling and loading C# code at runtime — review the code being compiled for safety."},
		{ID: "WIN-012", Name: "cmd_exe_c", Pattern: `(?i)cmd\s*/c\s+.*(del|rd|rmdir|format)\s`, Category: "shell_injection", Severity: "high", Description: "cmd.exe is running a destructive command (delete/format) — verify the target is correct before proceeding."},
		{ID: "WIN-013", Name: "powershell_download_string", Pattern: `(?i)\(new-object.*\)\.downloadstring\(`, Category: "shell_injection", Severity: "critical", Description: "PowerShell DownloadString fetches and typically executes remote code — download the script to a file first, review it, then run it."},
	}
}
