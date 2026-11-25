# Working in this repo

- Go, standard library only. No third-party modules without a good reason.
- `make test` must pass before calling anything done (`go vet` + unit tests).
- The CLI is `cmd/sitecheck/`; the Claude Code skill is `skills/site-check/`.
  Keep `SKILL.md` short: detail goes in `reference.md`, AEO evidence and
  judgment in `aeo.md`.
- The repo is also a Claude Code plugin and its own marketplace
  (`.claude-plugin/`). Run `claude plugin validate .` after touching those
  files. Don't add a top-level `bin/`: claude.ai won't install such plugins.
- Secrets and personal data (API keys, service-account files, absolute paths
  under a home folder) never go in the repo. They live in
  `~/.config/sitecheck/`. Examples go in `config/*.example*` with placeholders.
- AEO claims in `aeo.md` carry a confidence level and a source. Don't add a
  rule or recommendation the evidence doesn't support.
- External tools (trafilatura, advertools, Lighthouse) are pinned in
  `cmd/sitecheck/tools.go`; change the pins there and in the READMEs together.

## Tasks

Track work with `scripts/task` (one Markdown file per task in `tasks/`):

- `scripts/task` lists open tasks; `scripts/task list all` shows everything.
- When you find follow-up work, `scripts/task add "Title" <area>` rather than
  leaving a TODO comment.
- Mark progress with `scripts/task start <id>` / `scripts/task done <id>`, and
  record decisions or findings with `scripts/task note <id> "..."`.
