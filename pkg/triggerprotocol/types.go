package triggerprotocol

const ProtocolVersion = "bach.trigger.v1"

const (
	ErrorInvalidRequest      = "invalid_request"
	ErrorUnsupportedProtocol = "unsupported_protocol"
	ErrorValidationFailed    = "validation_failed"
	ErrorConflict            = "conflict"
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
const CapabilityAck Capability = "ack"
const CapabilityNack Capability = "nack"

type HandshakeParams struct {
	Protocol string            `json:"protocol"`
	Factory  string            `json:"factory"`
	Trigger  string            `json:"trigger"`
	Config   map[string]string `json:"config,omitempty"`
}

type HandshakeResult struct {
	Protocol     string       `json:"protocol"`
	Provider     string       `json:"provider"`
	Version      string       `json:"version"`
	Capabilities []Capability `json:"capabilities"`
}

type PollParams struct {
	Cursor string            `json:"cursor"`
	Config map[string]string `json:"config,omitempty"`
}

type PollResult struct {
	Cursor string     `json:"cursor"`
	Items  []PollItem `json:"items"`
}

type PollItem struct {
	Source   ItemSource        `json:"source"`
	Title    string            `json:"title"`
	Body     string            `json:"body,omitempty"`
	Labels   []string          `json:"labels,omitempty"`
	Priority string            `json:"priority,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ItemSource struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	URL      string `json:"url,omitempty"`
	Revision string `json:"revision,omitempty"`
}

type AckParams struct {
	Cursor    string   `json:"cursor"`
	SourceIDs []string `json:"source_ids,omitempty"`
}

type NackParams struct {
	Cursor string `json:"cursor"`
	Reason string `json:"reason"`
}
