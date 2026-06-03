package store

import (
	"context"
	"errors"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
)

var ErrNotFound = errors.New("store record not found")

type Store interface {
	SaveTask(context.Context, *tasks.Task) error
	Task(context.Context, string) (*tasks.Task, error)
	SaveRun(context.Context, *runs.Run) error
	Run(context.Context, string) (*runs.Run, error)
	ListRuns(context.Context) ([]runs.Run, error)
	SaveEvent(context.Context, *events.Event) error
	EventsByRun(context.Context, string) ([]events.Event, error)
	SaveArtifact(context.Context, *artifacts.Artifact) error
	ArtifactsByRun(context.Context, string) ([]artifacts.Artifact, error)
	ListArtifacts(context.Context, ArtifactListFilter) ([]artifacts.Artifact, error)
	PrunableArtifacts(context.Context, time.Time) ([]artifacts.PruneCandidate, error)
	DeleteArtifacts(context.Context, []string) error
	Close() error
}

type ArtifactListFilter struct {
	RunID  string
	Status runs.RunStatus
}
