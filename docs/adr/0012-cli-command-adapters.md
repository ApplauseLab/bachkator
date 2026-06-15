# CLI Command Adapters

`internal/cli` should own an internal command registry or composition point for **CLI Command Adapters**. This allows new subcommands and command groups to be developed in parallel while preserving a single owner for the public **CLI Contract** and Cobra tree. Core domain packages should expose Go functions and domain types, not Cobra commands, unless they are explicitly CLI adapter packages under the CLI layer.
