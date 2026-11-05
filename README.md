# denon-synology-vtuner

Notes and configuration for serving a Denon receiver's vTuner internet radio requests from a Synology NAS instead of the public vTuner service.

See [docs/](docs/) for tutorials.

## Development workflow

This project is spec-driven with [OpenSpec](https://github.com/Fission-AI/OpenSpec).

- `openspec/specs/` holds the current specs (what the system does).
- `openspec/changes/` holds proposed changes; finished ones move to `openspec/changes/archive/`.
- `openspec/config.yaml` holds the project context that guides every proposal.

Install the CLI with `npm install -g @fission-ai/openspec`, then start a change with `/opsx:propose "your idea"` in Claude Code (or `openspec new change <name>`). Run `openspec list` and `openspec validate --all` to check status.
