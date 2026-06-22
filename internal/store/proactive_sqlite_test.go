package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/schedule"
	"github.com/deon7769/deonclaw/internal/wakeup"
)

func TestClaimNextDueWakeupIsAtomic(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "claim.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	if err := store.SaveSchedule(ctx, schedule.Schedule{
		ID: "sched", Kind: schedule.KindEvery, Name: "sched", Status: schedule.StatusActive,
		AgentID: "agent", Every: "30m", CreatedAt: now.Format(time.RFC3339Nano),
		UpdatedAt: now.Format(time.RFC3339Nano), NextDueAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("SaveSchedule() error = %v", err)
	}
	w := wakeup.Wakeup{
		ID: "w1", ScheduleID: "sched", AgentID: "agent",
		DueAt:  now.Add(-time.Minute).Format(time.RFC3339Nano),
		Status: wakeup.StatusQueued, IdempotencyKey: "idem_test",
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	if err := store.SaveWakeup(ctx, w); err != nil {
		t.Fatalf("SaveWakeup() error = %v", err)
	}

	var wg sync.WaitGroup
	claims := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.ClaimNextDueWakeup(ctx, now)
			if err != nil {
				t.Errorf("ClaimNextDueWakeup() error = %v", err)
				return
			}
			claims <- ok
		}()
	}
	wg.Wait()
	close(claims)

	success := 0
	for ok := range claims {
		if ok {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful claims = %d, want 1", success)
	}

	items, err := store.ListWakeups(ctx)
	if err != nil {
		t.Fatalf("ListWakeups() error = %v", err)
	}
	if len(items) != 1 || items[0].Status != wakeup.StatusClaimed {
		t.Fatalf("wakeup after claim = %+v", items)
	}
}
