# CLI Contract over Public Go API

Bachkator's supported product interface is the **CLI Contract**: commands, flags, Bachfile syntax, embedded reference docs, and command output semantics. Go packages under `internal/` remain private implementation details so the loader, planner, runner, scheduler, state store, and quality ingestion code can evolve without committing to a library API. A public Go API should only be introduced for concrete embedder use cases that cannot be served by the CLI or external plugin contracts.
