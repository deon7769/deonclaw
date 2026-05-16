package artifacts

import "time"

type Kind string

const (
	KindSummary Kind = "summary"
	KindLog     Kind = "log"
	KindDiff    Kind = "diff"
	KindEvents  Kind = "events"
	KindOther   Kind = "other"
)

type Artifact struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Path      string    `json:"path"`
	Kind      Kind      `json:"kind"`
	Content   []byte    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
