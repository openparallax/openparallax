---
name: shell
description: Execute shell commands with security evaluation.
when_to_use: When the user wants to run a terminal command or script.
actions:
  - execute_command
keywords:
  - run
  - execute
  - command
  - shell
  - terminal
  - bash
  - script
emoji: "\U0001F4BB"
---

# Shell Execution

Run shell commands with automatic Shield evaluation. Commands are executed via the platform shell (/bin/sh on Unix, cmd.exe on Windows) with a 30-second timeout.
