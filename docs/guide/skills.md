# Skills

Skills are custom domain-specific guidance files that extend the agent's knowledge. Unlike tools (which execute actions), skills provide instructions and context that shape how the agent approaches a particular domain.

## How Skills Work

Skills follow a two-phase discovery and loading pattern:

1. **Discovery** — on startup, the agent scans `<workspace>/skills/` for SKILL.md files. It extracts each skill's name and description (from the YAML frontmatter) and includes a compact index in the system prompt.
2. **On-demand loading** — when the LLM determines a skill is relevant to the current conversation, it calls `load_skills` with the skill name. The full markdown body is then injected into the context.

This approach keeps the system prompt compact (only names and descriptions) while making detailed guidance available when needed.

## Directory Structure

Each skill lives in its own subdirectory under `<workspace>/skills/`:

```
<workspace>/
  skills/
    code-review/
      SKILL.md
    deployment/
      SKILL.md
      templates/
        fly.toml
    data-analysis/
      SKILL.md
```

The subdirectory name is for organization — the skill's canonical name comes from the `name` field in the YAML frontmatter.

### SKILL.md Format

Each SKILL.md file has two parts:

1. **YAML frontmatter** — enclosed in `---` delimiters, containing metadata
2. **Markdown body** — the actual instructions and guidance

```markdown
---
name: code-review
description: Guidelines for reviewing code — style, testing, security, and performance checks
---

# Code Review

## Style
- Prefer descriptive variable names over abbreviations
- Functions should do one thing
- Keep files under 300 lines

## Testing
- Every public function needs a test
- Integration tests for database operations
- Mock external services

## Security
- Never log secrets or API keys
- Validate all user input
- Use parameterized queries for SQL

## Performance
- Profile before optimizing
- Avoid N+1 queries
- Cache expensive computations
```

### Frontmatter Fields

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Skill identifier. Lowercase, hyphens allowed, max 64 characters. This is the name used in `load_skills` calls. | Yes |
| `description` | A concise explanation of what the skill covers and when to use it. The LLM sees this during discovery to decide whether to load the skill. Write it as if you are telling the agent when this skill is relevant. | Yes |

### Markdown Body

The body can contain any markdown content. This is injected verbatim into the LLM context when the skill is loaded. Write it as instructions for the agent, not as documentation for humans.

Effective skill bodies:

- **Be specific** — concrete rules and examples work better than vague principles
- **Be actionable** — tell the agent what to do, not what to know
- **Include examples** — show input/output pairs, code patterns, or templates
- **Stay focused** — one skill per domain, not a catch-all

## Discovery

When the agent starts, it reads all SKILL.md files and builds a discovery summary. This summary appears in the system prompt:

```
# Custom Skills

You have access to user-defined guidance for these domains:
- **code-review**: Guidelines for reviewing code — style, testing, security, and performance checks
- **deployment**: Steps for deploying applications to fly.io and Railway

To get detailed instructions for a domain, call load_skills with the skill name.
```

The LLM uses this index to decide which skills are relevant to the current conversation. It calls `load_skills` only when it determines a skill would help with the user's request.

## The load_skills Meta-Tool

`load_skills` is a built-in meta-tool that the agent calls to load skill content into its context. It takes a skill name and returns the full markdown body.

From the LLM's perspective:

```
User: Review this pull request for me

Agent thinking: The user wants a code review. I have a "code-review" skill available.
→ Calls load_skills("code-review")
→ Receives the full skill body
→ Applies the guidelines to the code review
```

Once a skill is loaded, its body stays in the context for the remainder of the session. The agent does not need to reload it for subsequent messages.

### Session Reset

Loaded skills are cleared when a new session starts. This prevents stale skill context from accumulating across unrelated conversations.

## Examples

### Writing Assistant

```markdown
---
name: writing
description: Guidelines for writing and editing prose — tone, structure, clarity, and formatting
---

# Writing Guidelines

## Tone
- Match the audience: technical docs are precise, blog posts are conversational
- Active voice by default
- Avoid jargon unless the audience expects it

## Structure
- Lead with the most important point
- One idea per paragraph
- Use headers for sections longer than 3 paragraphs

## Editing Checklist
1. Remove unnecessary words (very, really, just, basically)
2. Replace abstract nouns with concrete ones
3. Check that every paragraph has a clear topic sentence
4. Verify all claims have sources
```

### Git Workflow

```markdown
---
name: git-workflow
description: Team git conventions — branching strategy, commit messages, PR process, and release tagging
---

# Git Workflow

## Branching
- `main` is always deployable
- Feature branches: `feat/<ticket>-<description>`
- Bugfix branches: `fix/<ticket>-<description>`

## Commits
- Format: `type(scope): description`
- Types: feat, fix, refactor, test, docs, chore
- Keep the first line under 72 characters
- Reference ticket numbers in the body

## Pull Requests
- One logical change per PR
- Include screenshots for UI changes
- At least one approval required
- Squash merge to main

## Releases
- Tag format: `v<major>.<minor>.<patch>`
- Generate changelog from commit messages
- Deploy automatically on tag push
```

### Project-Specific Knowledge

```markdown
---
name: acme-api
description: ACME Corp internal API conventions — endpoints, authentication, error handling, and database patterns
---

# ACME API Conventions

## Endpoints
- RESTful: `GET /api/v1/resources`, `POST /api/v1/resources`
- Use plural nouns for collections
- Nest related resources: `/api/v1/users/{id}/orders`

## Authentication
- JWT tokens via Authorization header
- Refresh tokens stored in httpOnly cookies
- Token expiry: 15 minutes (access), 7 days (refresh)

## Error Responses
Always return:
```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "User with ID 123 not found",
    "details": {}
  }
}
```

## Database
- PostgreSQL with pgx driver
- Migrations in `db/migrations/` using golang-migrate
- Use transactions for multi-table operations
- Index foreign keys
```

### DevOps Runbook

```markdown
---
name: incident-response
description: Incident response procedures — severity levels, escalation, communication, and postmortem process
---

# Incident Response

## Severity Levels
- **SEV1**: Service down, data loss risk → page on-call, all hands
- **SEV2**: Degraded service, workaround exists → notify team lead
- **SEV3**: Minor issue, no user impact → fix in next sprint

## Escalation
1. Acknowledge within 5 minutes
2. Assess severity
3. SEV1/2: open incident channel, notify stakeholders
4. Begin investigation with logs and metrics
5. Document timeline in incident doc

## Communication
- Status page update within 15 minutes for SEV1
- Stakeholder update every 30 minutes during active incident
- Resolution notification to all affected parties

## Postmortem
- Required for SEV1 and SEV2
- Blameless — focus on systems, not people
- Template in Notion: "Incident Postmortem Template"
- Action items tracked in Jira with due dates
```

## Additional Resources in Skill Directories

Skill directories can contain additional files beyond SKILL.md. These are not automatically loaded but can be referenced by the agent using file tools:

```
skills/
  deployment/
    SKILL.md
    templates/
      fly.toml
      Dockerfile
    scripts/
      deploy.sh
```

The skill body can reference these files:

```markdown
## Deployment Template

Use the fly.toml template at `skills/deployment/templates/fly.toml` as a starting point.
Customize the `[env]` section for the target environment.
```

When the agent needs the template, it uses `read_file` to load it.

## Next Steps

- [Tools](/guide/tools) — the actions skills can reference
- [Memory](/guide/memory) — how skills interact with memory
- [Configuration](/guide/configuration) — workspace path where skills live
