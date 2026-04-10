// Package tier1 implements the dual classifier (ONNX + heuristic) for Shield's
// second evaluation tier. It detects shell injection, prompt injection, path
// traversal, data exfiltration, and other attack patterns.
package shield

import "github.com/openparallax/openparallax/platform"

// CrossPlatformDetectionRules returns rules that detect attacks independent of OS.
// These cover prompt injection, path traversal, data exfiltration, sensitive data
// exposure, and encoding evasion.
func CrossPlatformDetectionRules() []platform.HeuristicRule {
	return []platform.HeuristicRule{
		// Prompt injection (5 rules)
		{ID: "PI-001", Name: "ignore_instructions", Pattern: `(?i)ignore\s+(all\s+)?(previous|prior|above)\s+instructions`, Category: "prompt_injection", Severity: "critical", Description: "Instruction override attempt"},
		{ID: "PI-002", Name: "system_message_spoof", Pattern: `(?i)(system|admin|root)\s*:\s*(you are|your new|override|update your)`, Category: "prompt_injection", Severity: "critical", Description: "System message spoofing"},
		{ID: "PI-003", Name: "role_switch", Pattern: `(?i)(you are now|act as|pretend to be|switch to)\s+(a |an |the )?(hacker|admin|root|unrestricted)`, Category: "prompt_injection", Severity: "critical", Description: "Role switch attempt"},
		{ID: "PI-004", Name: "jailbreak_markers", Pattern: `(?i)(DAN|do anything now|developer mode|god mode|jailbreak)`, Category: "prompt_injection", Severity: "high", Description: "Jailbreak keyword detected"},
		{ID: "PI-005", Name: "instruction_override", Pattern: `(?i)(forget|disregard|override|bypass)\s+(your|all|every)\s+(rules|instructions|guidelines|constraints)`, Category: "prompt_injection", Severity: "critical", Description: "Instruction override attempt"},

		// Path traversal (3 rules; PT-001 escalates because nested ../
		// is sometimes legitimate in monorepo relative imports)
		{ID: "PT-001", Name: "dot_dot_traversal", Pattern: `\.\./\.\./`, Category: "path_traversal", Severity: "high", Description: "Directory traversal via ../ — sometimes legitimate in monorepos.", Escalate: true},
		{ID: "PT-002", Name: "null_byte", Pattern: `%00|\\x00|\\0`, Category: "path_traversal", Severity: "critical", Description: "Null byte injection"},
		{ID: "PT-003", Name: "url_encoded_traversal", Pattern: `%2[eE]%2[eE]/%2[eE]%2[eE]/`, Category: "path_traversal", Severity: "high", Description: "URL-encoded directory traversal"},

		// Data exfiltration — DE-001 (base64_in_url) and DE-002
		// (dns_exfil) were dropped: false positives on every signed
		// S3/JWT URL and any long subdomain (e.g. AWS internal hosts).
		{ID: "DE-003", Name: "webhook_exfil", Pattern: `(?i)(hooks\.slack\.com|discord\.com/api/webhooks)`, Category: "data_exfiltration", Severity: "high", Description: "Slack/Discord webhook — legitimate notification channels but worth a look.", Escalate: true},

		// Sensitive data (3 rules)
		{ID: "SD-001", Name: "private_key_content", Pattern: `-----BEGIN\s+(RSA|EC|OPENSSH|PGP)\s+PRIVATE\s+KEY-----`, Category: "sensitive_data", Severity: "critical", Description: "Private key content detected"},
		{ID: "SD-002", Name: "aws_key", Pattern: `AKIA[0-9A-Z]{16}`, Category: "sensitive_data", Severity: "critical", Description: "AWS access key detected"},
		{ID: "SD-003", Name: "jwt_token", Pattern: `eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`, Category: "sensitive_data", Severity: "high", Description: "JWT token detected — legitimate JWT handling code is common.", Escalate: true},

		// Encoding evasion (1 rule)
		{ID: "EE-001", Name: "zero_width_chars", Pattern: `[\x{200B}\x{200C}\x{200D}\x{FEFF}\x{00AD}]`, Category: "encoding_evasion", Severity: "high", Description: "Zero-width character detected"},

		// Self-protection (1 rule)
		{ID: "SP-001", Name: "shell_writes_protected_file", Pattern: `(?i)(>{1,2}\s*|tee\s+|cp\s+.*|mv\s+.*|rm\s+|del\s+|erase\s+|Set-Content\s+|Out-File\s+|Remove-Item\s+).*(SOUL\.md|IDENTITY\.md|TOOLS\.md|BOOT\.md)`, Category: "self_protection", Severity: "critical", Description: "Shell command writes to or deletes a protected identity file"},

		// Generation safety (3 rules)
		{ID: "GEN-001", Name: "gen_real_person_explicit", Pattern: `(?i)(nude|naked|explicit|nsfw|undress)\s.*(photo|image|picture|video)\s.*(of|with|featuring)\s`, Category: "generation_safety", Severity: "critical", Description: "Generation prompt requests explicit content of real person"},
		{ID: "GEN-002", Name: "gen_csam_adjacent", Pattern: `(?i)(child|minor|underage|teen|kid|boy|girl)\s.*(nude|naked|explicit|sexual|bath)`, Category: "generation_safety", Severity: "critical", Description: "Generation prompt contains CSAM-adjacent content"},
		{ID: "GEN-003", Name: "gen_weapons_visual", Pattern: `(?i)(how to (make|build|assemble)|blueprint|schematic|diagram).*(bomb|explosive|weapon|firearm|gun)`, Category: "generation_safety", Severity: "critical", Description: "Generation prompt requests weapons manufacturing visuals"},

		// Email safety (2 rules) — both escalate; moving to trash and
		// bulk flag changes are normal user actions.
		{ID: "EM-001", Name: "email_move_to_trash", Pattern: `(?i)email_move.*trash`, Category: "email_safety", Severity: "medium", Description: "Moving email to trash — normal user action; let evaluator judge.", Escalate: true},
		{ID: "EM-002", Name: "email_bulk_mark", Pattern: `(?i)email_mark.*(read|unread|flagged)`, Category: "email_safety", Severity: "low", Description: "Bulk email flag modification — normal user action; let evaluator judge.", Escalate: true},
	}
}
