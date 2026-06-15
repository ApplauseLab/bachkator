# Plan Execution Unit and Backend Plan Evidence

Bachkator v1 Plan execution should use one accepted Plan as the implementation unit. A Factory Work Item attempt may have one accepted Plan, and that Plan materializes one implementer Agent Target. Bach should not split one Plan into per-workstream agents in v1; when work is too large for one implementer, it should be split into multiple Plan files with Plan-level dependencies.

Plan identity is the slug used for dependency edges and CLI display. Bach may infer it from the project-relative Markdown path when frontmatter is absent, and Plan frontmatter may pin `id` when a stable identity must survive file moves. Backend Plan ledger records, evidence records, approvals, runs, and other exposed persisted records use Bach-generated UUIDv7 IDs. This keeps human Plan files readable while preserving stable public IDs for persistence, evidence, and managed-control-plane ingestion.

Bach-owned Plan ledger and Plan evidence records belong in the Project Backend database behind the Backend Provider protocol. Sidecar Plan ledger JSON files are not the authoritative storage model. The bundled SQLite Backend is the default local implementation, and future external Backends must preserve the same domain contract. Future Plan execution and Factory daemon phases write Plan ledger/evidence records through Backend methods; `bach plan status` reads those records rather than scanning private files.

The Plan domain package should remain side-effect free: infer defaults, parse optional frontmatter, validate Plan graphs, compute hashes, and derive status from supplied ledger DTOs. Backend loading and provider lifecycle belong in the composition/use-case layer, not inside the pure Plan parser or graph package.
