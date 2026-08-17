# Installing `opm-troubleshooting` in Claude Code (Native Marketplace)

This guide walks you through installing the **`opm-troubleshooting`** plugin into
Claude Code using the native plugin marketplace — no manual file copying, no config
editing required. When you're done, the plugin's slash commands
(`/opm-troubleshooting:inspect-bundle`, `/opm-troubleshooting:resolve-channel`, …) and
its RAG doc-search tools are available right inside your Claude Code session.

> **New here?** Once installed, see
> [Asking Questions and Searching the Docs](./asking-questions-and-searching-docs.md)
> for how to actually use the RAG knowledge base.

---

## The 30-second version

Run these two commands **inside a Claude Code session** (type them at the prompt):

```text
/plugin marketplace add midu16/opm-troubleshooting
/plugin install opm-troubleshooting@opm-troubleshooting
```

Then, if prompted, run `/reload-plugins`. That's it — type `/opm-troubleshooting:` at the
prompt and you'll see the commands.

The rest of this doc explains each step and how to verify, update, and remove it.

---

## Prerequisites

- **Claude Code CLI** installed and working (`claude --version`).
- **Network access** to GitHub (the marketplace is fetched from
  `github.com/midu16/opm-troubleshooting`).
- For the operator-inspection commands to actually run, you'll also want network access
  to pull OCI catalog images, and `DOCKER_CONFIG` set for private registries. That's a
  runtime concern, not an install concern — see the [README](../README.md).

---

## Step 1 — Add the marketplace

A "marketplace" is just a git repo containing a `.claude-plugin/marketplace.json`. This
repo *is* that marketplace. Point Claude Code at it with the GitHub `owner/repo`
shorthand:

```text
/plugin marketplace add midu16/opm-troubleshooting
```

What happens: Claude Code clones the repo, reads
[`.claude-plugin/marketplace.json`](../.claude-plugin/marketplace.json), and registers a
marketplace named **`opm-troubleshooting`** that offers one plugin, also named
**`opm-troubleshooting`**.

<details>
<summary>Alternative source forms</summary>

```text
# Full HTTPS git URL (note the https:// prefix and .git suffix)
/plugin marketplace add https://github.com/midu16/opm-troubleshooting.git

# SSH
/plugin marketplace add git@github.com:midu16/opm-troubleshooting.git

# Pin to a tag or branch
/plugin marketplace add https://github.com/midu16/opm-troubleshooting.git#v1.1.0

# From a local clone (the dir containing .claude-plugin/)
/plugin marketplace add /path/to/opm-troubleshooting
```

The `owner/repo` shorthand only works for GitHub. For other hosts, use the full
`https://…​.git` URL.
</details>

Confirm it registered:

```text
/plugin marketplace list
```

You should see `opm-troubleshooting` in the list.

## Step 2 — Install the plugin

Install by `<plugin-name>@<marketplace-name>` — here both happen to be
`opm-troubleshooting`:

```text
/plugin install opm-troubleshooting@opm-troubleshooting
```

Claude Code will prompt you to confirm and choose a scope (this user, or this project).
Pick whichever fits.

## Step 3 — Activate

Read the install summary:

- If it says **"Plugin is now active."** — you're done, nothing else needed.
- If it says **"Run /reload-plugins to activate."** — run:

  ```text
  /reload-plugins
  ```

  (If it warns that reloading invalidates the prompt cache and you want to force it:
  `/reload-plugins --force`.)

Commands and MCP servers bundled with the plugin become available after activation.

---

## Prefer the interactive menu?

Instead of typing the commands above, just run:

```text
/plugin
```

This opens a tabbed UI:

- **Discover** — browse plugins from all added marketplaces and install with **Enter**.
- **Installed** — enable / disable / uninstall plugins.
- **Marketplaces** — add, update, or remove marketplaces.
- **Errors** — see why a plugin failed to load.

Navigate with **Tab** / **Shift+Tab**. You still add the marketplace first (Step 1),
then install from the **Discover** tab.

---

## Verify it worked

```text
# The plugin should appear here
/plugin list
```

Then confirm the commands are live by typing the prefix at the prompt and letting
autocomplete show them:

```text
/opm-troubleshooting:
```

You should see:

- `/opm-troubleshooting:inspect-bundle`
- `/opm-troubleshooting:resolve-channel`
- `/opm-troubleshooting:batch-validate`
- `/opm-troubleshooting:telco-diagnose`
- `/opm-troubleshooting:adhd`
- `/opm-troubleshooting:rag-query` — search the docs/code/issues knowledge base
- `/opm-troubleshooting:rag-ingest` — build/refresh the knowledge base
- `/opm-troubleshooting:rag-server` — build/manage the `ocp-rag` MCP server

Quick smoke test:

```text
/opm-troubleshooting:resolve-channel \
  --catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 \
  --package odf-operator
```

> **Note on the RAG doc-search (MCP) tools:** the plugin also references an `ocp-rag`
> MCP server for documentation search. That server runs a locally-built binary and needs
> its knowledge base ingested first — installing the plugin does **not** do that for you.
> Follow [Asking Questions and Searching the Docs](./asking-questions-and-searching-docs.md)
> to build, configure, and populate it.

---

## Managing the plugin later

```text
# Refresh the marketplace catalog (pick up new versions)
/plugin marketplace update opm-troubleshooting

# Temporarily turn the plugin off / back on
/plugin disable opm-troubleshooting@opm-troubleshooting
/plugin enable  opm-troubleshooting@opm-troubleshooting

# Uninstall
/plugin uninstall opm-troubleshooting@opm-troubleshooting

# Remove the marketplace entirely
/plugin marketplace remove opm-troubleshooting
```

You can update to the latest version from your shell (outside a session) with:

```bash
claude plugin update opm-troubleshooting@opm-troubleshooting
```

---

## Install via `settings.json` (teams / repeatable setup)

To have the plugin enabled automatically — e.g. for everyone who opens a given project —
declare it in `.claude/settings.json` instead of running the commands by hand:

```json
{
  "extraKnownMarketplaces": {
    "opm-troubleshooting": {
      "source": {
        "source": "github",
        "repo": "midu16/opm-troubleshooting"
      }
    }
  },
  "enabledPlugins": [
    {
      "marketplace": "opm-troubleshooting",
      "plugin": "opm-troubleshooting"
    }
  ]
}
```

Commit this to the project's `.claude/settings.json` and every collaborator gets the
plugin on their next session (they'll be asked to trust the marketplace the first time).

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `invalid owner/repo` or clone error on a non-GitHub URL | Use the full `https://…​.git` URL with both the `https://` prefix and `.git` suffix. |
| Marketplace added but no plugin to install | Run `/plugin marketplace list` to confirm the name, then `/plugin install opm-troubleshooting@opm-troubleshooting`. |
| Commands don't show up after install | Run `/reload-plugins` (or `/reload-plugins --force`). Check the **Errors** tab in `/plugin`. |
| RAG / doc-search tools missing or empty | Expected — the MCP server needs building + ingesting. See the [RAG guide](./asking-questions-and-searching-docs.md). |
| Want to see load errors | `/plugin` → **Errors** tab. |

---

## See also

- [README — Claude Code Plugin install](../README.md#option-2-claude-code-plugin)
- [Asking Questions and Searching the Docs](./asking-questions-and-searching-docs.md)
- [docs/WORKFLOWS.md](./WORKFLOWS.md) — CI/release workflows
- Official docs: [Discover and install plugins](https://code.claude.com/docs/en/discover-plugins.md), [Plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces.md)
