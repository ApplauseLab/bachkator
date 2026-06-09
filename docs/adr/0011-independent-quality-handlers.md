# Independent Quality Handlers

Quality report parsers and quality gates should live in an independent `internal/quality` subsystem with registered **Quality Handlers**. Targets declare reports and gates, and the runner invokes quality ingestion as part of target completion, but target kinds should not own report parsing or gate evaluation. This keeps new report formats and gate types parallelizable without touching shell/image/pipeline/future target-kind implementations.
