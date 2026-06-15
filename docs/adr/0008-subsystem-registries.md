# Subsystem Registries

Bachkator's internal extension points should be owned by subsystem-specific registries rather than one central registry. Config decoding, target-kind specification, runner execution handlers, quality report/gate handlers, and CLI subcommands may each have their own registry, while a small composition root wires the built-ins together. This keeps parallel feature lanes independent and avoids replacing today's `internal/build` hotspot with a global registry hotspot.
