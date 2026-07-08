package beads

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// fakeRunner records the argv it was called with and returns canned output.
type fakeRunner struct {
	gotArgs []string
	output  []byte
	err     error
}

func (f *fakeRunner) Run(args []string) ([]byte, error) {
	f.gotArgs = args
	return f.output, f.err
}

const issueFixture = `[
  {
    "id": "blf-bnt.1",
    "title": "beads adapter",
    "description": "build the read side",
    "status": "in_progress",
    "priority": 1,
    "issue_type": "task",
    "created_at": "2026-07-07T07:45:05Z",
    "updated_at": "2026-07-07T09:27:31Z",
    "started_at": "2026-07-07T08:16:33Z",
    "labels": ["ready-for-agent"],
    "parent": "blf-bnt",
    "dependency_count": 1,
    "dependent_count": 1
  }
]`

const depTreeFixture = `[
  {
    "id": "blf-bnt.1",
    "title": "beads adapter",
    "status": "in_progress",
    "priority": 1,
    "issue_type": "task",
    "created_at": "2026-07-07T07:45:05Z",
    "updated_at": "2026-07-07T09:27:31Z",
    "depth": 0
  },
  {
    "id": "blf-bnt",
    "title": "blf beads epic",
    "status": "open",
    "priority": 1,
    "issue_type": "epic",
    "created_at": "2026-07-07T07:43:30Z",
    "updated_at": "2026-07-07T07:43:30Z",
    "depth": 1,
    "parent_id": "blf-bnt.1",
    "edge_from_parent": "parent-child"
  }
]`

func wantIssue() Issue {
	created := mustParseTime("2026-07-07T07:45:05Z")
	updated := mustParseTime("2026-07-07T09:27:31Z")
	started := mustParseTime("2026-07-07T08:16:33Z")
	return Issue{
		ID:              "blf-bnt.1",
		Title:           "beads adapter",
		Description:     "build the read side",
		Status:          "in_progress",
		Priority:        1,
		IssueType:       "task",
		Labels:          []string{"ready-for-agent"},
		Parent:          "blf-bnt",
		DependencyCount: 1,
		DependentCount:  1,
		CreatedAt:       created,
		UpdatedAt:       updated,
		StartedAt:       &started,
	}
}

func TestList_argvAndDecode(t *testing.T) {
	tests := []struct {
		name     string
		scope    Scope
		dir      string
		wantArgs []string
	}{
		{
			name:     "actionable scope, no dir",
			scope:    ScopeActionable,
			wantArgs: []string{"list", "--json", "--limit", "0"},
		},
		{
			name:     "ready scope",
			scope:    ScopeReady,
			wantArgs: []string{"list", "--json", "--limit", "0", "--ready"},
		},
		{
			name:     "blocked scope",
			scope:    ScopeBlocked,
			wantArgs: []string{"list", "--json", "--limit", "0", "--status", "blocked"},
		},
		{
			name:     "all scope",
			scope:    ScopeAll,
			wantArgs: []string{"list", "--json", "--limit", "0", "--all"},
		},
		{
			name:     "threads -C dir before the op",
			scope:    ScopeActionable,
			dir:      "/tmp/some-project",
			wantArgs: []string{"-C", "/tmp/some-project", "list", "--json", "--limit", "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{output: []byte(issueFixture)}
			a := &Adapter{Runner: runner, Dir: tt.dir}

			issues, err := a.List(tt.scope)
			if err != nil {
				t.Fatalf("List: unexpected error: %v", err)
			}
			if !reflect.DeepEqual(runner.gotArgs, tt.wantArgs) {
				t.Errorf("argv = %v, want %v", runner.gotArgs, tt.wantArgs)
			}
			if len(issues) != 1 || !reflect.DeepEqual(issues[0], wantIssue()) {
				t.Errorf("decoded issue = %+v, want %+v", issues, wantIssue())
			}
		})
	}
}

func TestList_unknownScope(t *testing.T) {
	runner := &fakeRunner{output: []byte(`[]`)}
	a := &Adapter{Runner: runner}
	if _, err := a.List(Scope("nonsense")); err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestReady_argvAndIDSet(t *testing.T) {
	runner := &fakeRunner{output: []byte(issueFixture)}
	a := &Adapter{Runner: runner, Dir: "/proj"}

	set, err := a.Ready()
	if err != nil {
		t.Fatalf("Ready: unexpected error: %v", err)
	}
	wantArgs := []string{"-C", "/proj", "ready", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
	if !set["blf-bnt.1"] || len(set) != 1 {
		t.Errorf("id set = %v, want {blf-bnt.1}", set)
	}
}

func TestShow_argvAndDecode(t *testing.T) {
	runner := &fakeRunner{output: []byte(issueFixture)}
	a := &Adapter{Runner: runner}

	issue, err := a.Show("blf-bnt.1")
	if err != nil {
		t.Fatalf("Show: unexpected error: %v", err)
	}
	wantArgs := []string{"show", "blf-bnt.1", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
	if !reflect.DeepEqual(issue, wantIssue()) {
		t.Errorf("decoded issue = %+v, want %+v", issue, wantIssue())
	}
}

func TestShow_notFound(t *testing.T) {
	runner := &fakeRunner{output: []byte(`[]`)}
	a := &Adapter{Runner: runner}
	if _, err := a.Show("missing"); err == nil {
		t.Fatal("expected error for empty show result")
	}
}

func TestDepTree_argvAndDecode(t *testing.T) {
	tests := []struct {
		name      string
		direction DepDirection
		dir       string
		wantArgs  []string
	}{
		{
			name:      "down (default)",
			direction: DepDown,
			wantArgs:  []string{"dep", "tree", "blf-bnt.1", "--json", "--show-all-paths"},
		},
		{
			name:      "up",
			direction: DepUp,
			wantArgs:  []string{"dep", "tree", "blf-bnt.1", "--json", "--show-all-paths", "--direction", "up"},
		},
		{
			name:      "threads -C dir before the op",
			direction: DepDown,
			dir:       "/proj",
			wantArgs:  []string{"-C", "/proj", "dep", "tree", "blf-bnt.1", "--json", "--show-all-paths"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{output: []byte(depTreeFixture)}
			a := &Adapter{Runner: runner, Dir: tt.dir}

			nodes, err := a.DepTree("blf-bnt.1", tt.direction)
			if err != nil {
				t.Fatalf("DepTree: unexpected error: %v", err)
			}
			if !reflect.DeepEqual(runner.gotArgs, tt.wantArgs) {
				t.Errorf("argv = %v, want %v", runner.gotArgs, tt.wantArgs)
			}
			if len(nodes) != 2 {
				t.Fatalf("expected 2 nodes, got %d", len(nodes))
			}
			if nodes[1].ID != "blf-bnt" || nodes[1].Depth != 1 || nodes[1].ParentID != "blf-bnt.1" || nodes[1].EdgeFromParent != "parent-child" {
				t.Errorf("node[1] = %+v, want id=blf-bnt depth=1 parent=blf-bnt.1 edge=parent-child", nodes[1])
			}
		})
	}
}

const createdIssueFixture = `{
  "id": "blf-bnt.9",
  "title": "new issue",
  "status": "open",
  "priority": 2,
  "issue_type": "task",
  "created_at": "2026-07-07T07:43:30Z",
  "updated_at": "2026-07-07T07:43:30Z"
}`

const updatedIssueListFixture = `[
  {
    "id": "blf-bnt.1",
    "title": "beads adapter",
    "status": "in_progress",
    "priority": 1,
    "issue_type": "task",
    "created_at": "2026-07-07T07:45:05Z",
    "updated_at": "2026-07-07T09:27:31Z"
  }
]`

func TestCreate_argvAndDecode(t *testing.T) {
	tests := []struct {
		name     string
		opts     CreateOptions
		dir      string
		wantArgs []string
	}{
		{
			name:     "bare title",
			wantArgs: []string{"create", "new issue", "--json"},
		},
		{
			name:     "with parent, priority, type",
			opts:     CreateOptions{Parent: "blf-bnt", Priority: "1", Type: "task"},
			wantArgs: []string{"create", "new issue", "--json", "--parent", "blf-bnt", "--priority", "1", "--type", "task"},
		},
		{
			name:     "threads -C dir before the op",
			dir:      "/proj",
			wantArgs: []string{"-C", "/proj", "create", "new issue", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{output: []byte(createdIssueFixture)}
			a := &Adapter{Runner: runner, Dir: tt.dir}

			issue, err := a.Create("new issue", tt.opts)
			if err != nil {
				t.Fatalf("Create: unexpected error: %v", err)
			}
			if !reflect.DeepEqual(runner.gotArgs, tt.wantArgs) {
				t.Errorf("argv = %v, want %v", runner.gotArgs, tt.wantArgs)
			}
			if issue.ID != "blf-bnt.9" || issue.Title != "new issue" {
				t.Errorf("decoded issue = %+v, want id=blf-bnt.9 title=%q", issue, "new issue")
			}
		})
	}
}

func TestUpdateStatus_argvAndDecode(t *testing.T) {
	runner := &fakeRunner{output: []byte(updatedIssueListFixture)}
	a := &Adapter{Runner: runner, Dir: "/proj"}

	issue, err := a.UpdateStatus("blf-bnt.1", "in_progress")
	if err != nil {
		t.Fatalf("UpdateStatus: unexpected error: %v", err)
	}
	wantArgs := []string{"-C", "/proj", "update", "blf-bnt.1", "--status", "in_progress", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
	if issue.Status != "in_progress" {
		t.Errorf("decoded issue status = %q, want in_progress", issue.Status)
	}
}

func TestUpdateStatus_noIssueReturned(t *testing.T) {
	runner := &fakeRunner{output: []byte(`[]`)}
	a := &Adapter{Runner: runner}
	if _, err := a.UpdateStatus("missing", "closed"); err == nil {
		t.Fatal("expected error for empty update result")
	}
}

func TestClose_argv(t *testing.T) {
	runner := &fakeRunner{output: []byte(updatedIssueListFixture)}
	a := &Adapter{Runner: runner}

	if _, err := a.Close("blf-bnt.1"); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
	wantArgs := []string{"close", "blf-bnt.1", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
}

func TestReopen_argv(t *testing.T) {
	runner := &fakeRunner{output: []byte(updatedIssueListFixture)}
	a := &Adapter{Runner: runner}

	if _, err := a.Reopen("blf-bnt.1"); err != nil {
		t.Fatalf("Reopen: unexpected error: %v", err)
	}
	wantArgs := []string{"reopen", "blf-bnt.1", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
}

func TestGraph_argv(t *testing.T) {
	tests := []struct {
		name     string
		format   GraphFormat
		wantArgs []string
	}{
		{
			name:     "compact",
			format:   GraphCompact,
			wantArgs: []string{"graph", "blf-bnt.1", "--compact"},
		},
		{
			name:     "html",
			format:   GraphHTML,
			wantArgs: []string{"graph", "blf-bnt.1", "--html"},
		},
		{
			name:     "default is compact",
			format:   "",
			wantArgs: []string{"graph", "blf-bnt.1", "--compact"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{output: []byte("graph output")}
			a := &Adapter{Runner: runner}

			out, err := a.Graph("blf-bnt.1", tt.format)
			if err != nil {
				t.Fatalf("Graph: unexpected error: %v", err)
			}
			if !reflect.DeepEqual(runner.gotArgs, tt.wantArgs) {
				t.Errorf("argv = %v, want %v", runner.gotArgs, tt.wantArgs)
			}
			if string(out) != "graph output" {
				t.Errorf("output = %q, want %q", out, "graph output")
			}
		})
	}
}

func TestGraph_unknownFormat(t *testing.T) {
	runner := &fakeRunner{output: []byte("")}
	a := &Adapter{Runner: runner}
	if _, err := a.Graph("blf-bnt.1", GraphFormat("nonsense")); err == nil {
		t.Fatal("expected error for unknown graph format")
	}
}

func TestGraphIssueCommand_UsesPagerWhenAvailable(t *testing.T) {
	cmd := graphIssueCommand("/proj", "blf-bnt.1")

	if got, want := cmd.Path, "/bin/sh"; got != want && got != "sh" {
		t.Fatalf("Path = %q, want shell", got)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "less -R") {
		t.Fatalf("command = %q, want less pager", joined)
	}
	if !strings.Contains(joined, `bd graph "$2" --compact`) {
		t.Fatalf("command = %q, want compact graph shell-out", joined)
	}
}
