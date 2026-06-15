# Architecture Phase before Parallel Features

Bachkator should perform a behavior-preserving architecture phase before broad parallel feature development. The phase should be sliced by subsystem, starting with `internal/model`, then independent quality/state seams, then runner, config, app composition, and CLI command adapters. This reduces merge-conflict pressure in the current `internal/build` package and creates the package and registry boundaries needed for multiple target-kind, config, quality, and CLI-command features to proceed safely in parallel.
