# Contributing New Agents

When adding new agents:

1. **Define clear use case requiring AI reasoning** (not simple scripting) — see [design principles](design-principles.md) for the "no pure automation" rule.
2. Document workflow steps and prompt template in this file or a dedicated doc under `docs/agents/`.
3. If the agent should be accessible as a slash command, create markdown in `commands/` directory and register it in `.claude-plugin/plugin.json`.
4. Test against real catalog scenarios.
5. Update root `agents/AGENTS.md` quick-command table if adding a new slash command.

## Checklist for New Agent PR

- [ ] Agent does more than wrap a single tool call (see [design principles](design-principles.md))
- [ ] Workflow is documented with numbered steps
- [ ] Prompt template uses `{placeholders}` consistently
- [ ] Output follows the standard format (see [output-format.md](output-format.md))
- [ ] Invocation examples use real commands (slash commands from `commands/` or CLI binaries from `bin/`)
- [ ] If adding a slash command: registered in `.claude-plugin/plugin.json` and listed in `commands/` directory
- [ ] Root `agents/AGENTS.md` quick-command table is updated (if applicable)

## Note on Command Types

This project has two types of commands:

1. **Plugin Slash Commands** — Registered in `.claude-plugin/plugin.json`, accessible via `/command-name`. Load markdown from `commands/*.md`. Examples: `/inspect-bundle`, `/resolve-channel`, `/batch-validate`, `/adhd`.

2. **CLI Binaries** — Executable Go binaries in `bin/`. Run directly: `./bin/telco-diagnose --must-gather ...`. Examples: `telco-diagnose`, `catalog-bundle-inspect`, `opm-diagnose`.
