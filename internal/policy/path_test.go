package policy

import (
	"strings"
	"testing"
)

func TestChangedPathsFromGitDiff(t *testing.T) {
	diff := []byte(`diff --git a/internal/tasks/task.go b/internal/tasks/task.go
index 1111111..2222222 100644
--- a/internal/tasks/task.go
+++ b/internal/tasks/task.go
@@ -1 +1 @@
-old
+new
diff --git a/secrets/token.txt b/secrets/token.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/secrets/token.txt
@@ -0,0 +1 @@
+token
`)

	got := ChangedPathsFromGitDiff(diff)
	want := []string{"internal/tasks/task.go", "secrets/token.txt"}
	if len(got) != len(want) {
		t.Fatalf("ChangedPathsFromGitDiff() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ChangedPathsFromGitDiff()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEvaluateChangedPathsRejectsForbiddenPath(t *testing.T) {
	result := EvaluateChangedPaths("workspace_write", []string{"secrets/token.txt"}, []string{"internal/**"}, []string{"secrets/**"})

	if result.OK() {
		t.Fatal("EvaluateChangedPaths() OK = true, want forbidden violation")
	}
	if !strings.Contains(result.Summary(), `secrets/token.txt matches forbidden path "secrets/**"`) {
		t.Fatalf("summary = %q, want forbidden path detail", result.Summary())
	}
}

func TestEvaluateChangedPathsRejectsPathOutsideAllowedPaths(t *testing.T) {
	result := EvaluateChangedPaths("workspace_write", []string{"README.md"}, []string{"internal/**"}, []string{"secrets/**"})

	if result.OK() {
		t.Fatal("EvaluateChangedPaths() OK = true, want allowed path violation")
	}
	if !strings.Contains(result.Summary(), "README.md is outside allowed paths") {
		t.Fatalf("summary = %q, want allowed path detail", result.Summary())
	}
}

func TestEvaluateChangedPathsRejectsReadOnlyDiff(t *testing.T) {
	result := EvaluateChangedPaths("read_only", []string{"internal/tasks/task.go"}, nil, []string{"secrets/**"})

	if result.OK() {
		t.Fatal("EvaluateChangedPaths() OK = true, want read_only violation")
	}
	if !strings.Contains(result.Summary(), "read_only task changed files") {
		t.Fatalf("summary = %q, want read_only detail", result.Summary())
	}
}

func TestEvaluateChangedPathsAllowsAllowedPath(t *testing.T) {
	result := EvaluateChangedPaths("workspace_write", []string{"internal/tasks/task.go"}, []string{"internal/**"}, []string{"secrets/**"})

	if !result.OK() {
		t.Fatalf("EvaluateChangedPaths() summary = %q, want OK", result.Summary())
	}
	if result.Summary() != "ok" {
		t.Fatalf("Summary() = %q, want ok", result.Summary())
	}
}
