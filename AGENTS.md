# Agent Guidelines

## Language policy

- Communicate with the user in **Thai**: conversation, explanations, summaries, and questions.
- Everything that gets implemented must be in **English**: code identifiers, code comments, commit messages, documentation (README, ADRs, API docs), `.env.example` descriptions, test names, and log/error messages.
- Exception: user-facing product UI copy in `web/` stays bilingual (Thai/English) by design — do not translate it as part of this policy.

## Terminal policy

- Never leave background processes running when the work is done. Stop every dev server, watcher, or long-running command you started (verify the ports are actually free — `go run` children can outlive their wrapper).
- The user runs and verifies the app themselves (e.g. `make dev`). Do not keep servers up "for convenience"; closing the terminal after finishing is mandatory, every time.
