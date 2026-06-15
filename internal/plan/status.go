package plan

import "strings"

func DeriveStatuses(selection Selection, ledgers map[string]Ledger) []StatusRecord {
	statuses := make(map[string]string, len(selection.Documents))
	records := make([]StatusRecord, 0, len(selection.Documents))
	byID := make(map[string]Document, len(selection.Documents))
	for _, doc := range selection.Documents {
		byID[doc.ID] = doc
	}
	for _, wave := range selection.Waves {
		for _, id := range wave {
			doc := byID[id]
			record := StatusRecord{Document: doc, Status: StatusReady}
			if ledger, ok := ledgers[id]; ok {
				record.Ledger = &LedgerSummary{
					LedgerID:   ledger.LedgerID,
					Status:     ledger.Status,
					Hash:       ledger.Hash,
					RecordedAt: ledger.RecordedAt,
				}
				diagnostics := ValidateLedger(doc, ledger)
				switch {
				case len(diagnostics) > 0:
					record.Status = StatusInvalidLedger
					record.Diagnostics = diagnostics
				case ledger.Status == StatusImplemented && ledger.Hash == doc.Hash:
					record.Status = StatusImplemented
				case ledger.Status == StatusImplemented:
					record.Status = StatusStale
				case ledger.Status == StatusPending || ledger.Status == StatusInProgress ||
					ledger.Status == StatusFailed:
					record.Status = ledger.Status
				default:
					record.Status = StatusInvalidLedger
					record.Diagnostics = []Diagnostic{
						diag(
							doc.Path,
							"invalid-ledger-status",
							"Plan ledger status is not supported",
						),
					}
				}
			} else if len(doc.DependsOn) > 0 {
				record.Status = StatusPlanned
				for _, dep := range doc.DependsOn {
					depStatus := statuses[dep]
					if depStatus == StatusBlocked || depStatus == StatusInvalidLedger ||
						depStatus == StatusStale {
						record.Status = StatusBlocked
						break
					}
					if depStatus != StatusImplemented {
						record.Status = StatusPlanned
					}
				}
			}
			statuses[id] = record.Status
			records = append(records, record)
		}
	}
	return records
}

func ValidateLedger(doc Document, ledger Ledger) []Diagnostic {
	diagnostics := []Diagnostic{}
	if ledger.SchemaVersion != LedgerSchemaVersion {
		diagnostics = append(
			diagnostics,
			diag(
				doc.Path,
				"invalid-ledger-schema",
				"Plan ledger schema must be "+LedgerSchemaVersion,
			),
		)
	}
	if ledger.LedgerID == "" {
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "missing-ledger-id", "Plan ledger ID is empty"),
		)
	}
	if ledger.PlanID != doc.ID {
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "ledger-plan-mismatch", "Plan ledger belongs to "+ledger.PlanID),
		)
	}
	switch ledger.Status {
	case StatusPending, StatusInProgress, StatusImplemented, StatusFailed:
	default:
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "invalid-ledger-status", "Plan ledger status must be implemented"),
		)
	}
	if !strings.HasPrefix(ledger.Hash, "sha256:") {
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "invalid-ledger-hash", "Plan ledger hash must start with sha256:"),
		)
	}
	if ledger.RecordedAt.IsZero() {
		diagnostics = append(
			diagnostics,
			diag(doc.Path, "invalid-ledger-recorded-at", "Plan ledger recorded_at is required"),
		)
	}
	for _, evidence := range ledger.Evidence {
		if evidence.ID == "" {
			diagnostics = append(
				diagnostics,
				diag(doc.Path, "missing-evidence-id", "Plan evidence ID is empty"),
			)
		}
		if evidence.Kind == "" {
			diagnostics = append(
				diagnostics,
				diag(doc.Path, "missing-evidence-kind", "Plan evidence kind is empty"),
			)
		}
	}
	return diagnostics
}
