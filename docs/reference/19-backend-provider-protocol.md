## Backend Provider Protocol

Bach Backend Providers speak `bach.backend.v1` over stdio JSON-RPC with `Content-Length` framing. Headers are capped at 16 KiB and payloads are capped at 8 MiB.

The bundled provider entrypoint is:

```sh
bach backend sqlite
```

Provider stdin and stdout are protocol-only. Diagnostics belong on stderr.

Initialization uses `backend.initialize` with the protocol version, Project name/root, and provider config. The bundled SQLite provider supports the capabilities `runs`, `evidence_refs`, `quality_reports`, `findings`, `factory_queue`, and `plan_ledger`.

Current method names are:

- `backend.initialize`
- `backend.shutdown`
- `runs.create`
- `runs.startTarget`
- `runs.finishTarget`
- `runs.finish`
- `runs.get`
- `runs.list`
- `evidence.recordRef`
- `evidence.listRefs`
- `quality.recordReport`
- `quality.recordReports`
- `findings.recordObservation`
- `findings.get`
- `findings.listCurrent`
- `findings.listEvents`
- `factory.enqueueWorkItem`
- `factory.getWorkItem`
- `factory.listWorkItems`
- `factory.cancelWorkItem`
- `factory.acquireDaemonLease`
- `factory.renewDaemonLease`
- `factory.releaseDaemonLease`
- `factory.claimWorkItem`
- `factory.updateWorkItemPhase`
- `factory.completeWorkItem`
- `factory.failWorkItem`
- `factory.getDaemonStatus`
- `plans.recordLedger`
- `plans.getLedger`

`runs.finish` accepts a run finish payload with:

- `run`: the final Run record.
- `targets`: changed target fingerprint records keyed by Target Address.
- `target_runs`: per-target execution records keyed by Target Address.
- `evidence`: evidence references associated with the completed Run.

Evidence references used by `runs.finish`, `evidence.recordRef`, and `evidence.listRefs` follow
`bach.backend.evidence_ref.v1`. They may include `created_at` as an RFC3339Nano timestamp; providers stamp
omitted values at write time.

`quality.recordReports` accepts a batch payload with `reports` and `gates` arrays. `quality.recordReport` accepts a single report.

`factory.enqueueWorkItem` accepts a Work Item, initial attempt, submitted event, and optional dedupe event.
`factory.getWorkItem` and `factory.cancelWorkItem` operate by factory name and Work Item ID.
`factory.listWorkItems` operates by factory name with an optional lifecycle status filter.
Daemon methods acquire and renew Factory leases, claim pending Work Items, record phase status, complete or
fail Work Items, and read daemon status without exposing table-level writes.
Work Item JSON follows `docs/schemas/backend-factory-work-item-v1.schema.json`.

`plans.recordLedger` appends one immutable Plan ledger and its evidence entries transactionally. Repeating the same `ledger_id` with an identical payload is idempotent; repeating it with a different payload returns `conflict`. `plans.getLedger` takes `plan_id` and returns the latest ledger by `recorded_at`, then `ledger_id`, or `not_found` when no ledger exists. Plan ledger JSON follows `docs/schemas/backend-plan-ledger-v1.schema.json`.

The `approvals` capability name is reserved for future approval operations; the current bundled provider does
not advertise it.

Public provider DTOs and helpers are available from:

- `github.com/applauselab/bachkator/pkg/backendprotocol`
- `github.com/applauselab/bachkator/pkg/jsonrpcstdio`

Machine-readable schemas live under `docs/schemas/backend-*.schema.json`.

Domain errors are JSON-RPC errors with Bach error codes in `error.data.code`, including `invalid_request`, `not_initialized`, `unsupported_capability`, `not_found`, `conflict`, `validation_failed`, and `internal`.
