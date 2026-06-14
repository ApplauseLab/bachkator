package approvalprotocol

const ProtocolVersion = "bach.approval.v1"

const (
	ErrorInvalidRequest      = "invalid_request"
	ErrorUnsupportedProtocol = "unsupported_protocol"
	ErrorValidationFailed    = "validation_failed"
	ErrorInternal            = "internal"
)

type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e Error) Error() string {
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

func NewError(code, message string) Error {
	return Error{Code: code, Message: message}
}

type Capability string

const CapabilityPoll Capability = "poll"

type HandshakeParams struct {
	Protocol string            `json:"protocol"`
	Factory  string            `json:"factory"`
	Approval string            `json:"approval"`
	Config   map[string]string `json:"config,omitempty"`
}

type HandshakeResult struct {
	Protocol     string       `json:"protocol"`
	Provider     string       `json:"provider"`
	Version      string       `json:"version"`
	Capabilities []Capability `json:"capabilities"`
}

type PollParams struct {
	Config map[string]string `json:"config,omitempty"`
}

type PollResult struct {
	Approvals []ApprovalRecord `json:"approvals"`
}

// ApprovalRecord is one externally observed approval awaiting validation by
// Bach. Exactly one of WorkItemID or SourceType+SourceID must be set; Phase
// must be the canonical gated phase such as plan or deploy.production.
type ApprovalRecord struct {
	WorkItemID string            `json:"work_item_id,omitempty"`
	SourceType string            `json:"source_type,omitempty"`
	SourceID   string            `json:"source_id,omitempty"`
	Phase      string            `json:"phase"`
	Rejected   bool              `json:"rejected,omitempty"`
	HeadRef    string            `json:"head_ref,omitempty"`
	Approver   string            `json:"approver,omitempty"`
	ApprovedAt string            `json:"approved_at,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}
