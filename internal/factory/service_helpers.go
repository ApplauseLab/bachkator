package factory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/applauselab/bachkator/internal/model"
)

type intakeSnapshot struct {
	SchemaVersion     string         `json:"schema_version"`
	WorkItemID        string         `json:"work_item_id"`
	Factory           string         `json:"factory"`
	Workflow          string         `json:"workflow"`
	Title             string         `json:"title"`
	Body              string         `json:"body,omitempty"`
	BodyHash          string         `json:"body_hash,omitempty"`
	Priority          model.Priority `json:"priority"`
	Labels            []string       `json:"labels,omitempty"`
	SourceType        string         `json:"source_type"`
	SourceID          string         `json:"source_id,omitempty"`
	SourceURL         string         `json:"source_url,omitempty"`
	SourceRevision    string         `json:"source_revision,omitempty"`
	DedupeKey         string         `json:"dedupe_key,omitempty"`
	SubmittedPlanPath string         `json:"submitted_plan_path,omitempty"`
	CreatedAt         string         `json:"created_at"`
}

func marshalIntake(value intakeSnapshot) ([]byte, string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, "", err
	}
	data = append(data, '\n')
	return data, sha256Bytes(data), nil
}

func sha256Text(value string) string {
	return sha256Bytes([]byte(value))
}

func sha256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
