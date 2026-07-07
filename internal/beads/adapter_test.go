package beads

import (
	"errors"
	"reflect"
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

const depFixture = `[
  {
    "id": "blf-bnt",
    "title": "blf beads epic",
    "status": "open",
    "priority": 1,
    "issue_type": "epic",
    "created_at": "2026-07-07T07:43:30Z",
    "updated_at": "2026-07-07T07:43:30Z",
    "dependency_type": "parent-child"
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
			wantArgs: []string{"list", "--json"},
		},
		{
			name:     "ready scope",
			scope:    ScopeReady,
			wantArgs: []string{"list", "--json", "--ready"},
		},
		{
			name:     "blocked scope",
			scope:    ScopeBlocked,
			wantArgs: []string{"list", "--json", "--status", "blocked"},
		},
		{
			name:     "all scope",
			scope:    ScopeAll,
			wantArgs: []string{"list", "--json", "--all"},
		},
		{
			name:     "threads -C dir before the op",
			scope:    ScopeActionable,
			dir:      "/tmp/some-project",
			wantArgs: []string{"-C", "/tmp/some-project", "list", "--json"},
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

func TestChildren_argv(t *testing.T) {
	runner := &fakeRunner{output: []byte(issueFixture)}
	a := &Adapter{Runner: runner, Dir: "/proj"}

	issues, err := a.Children("blf-bnt")
	if err != nil {
		t.Fatalf("Children: unexpected error: %v", err)
	}
	wantArgs := []string{"-C", "/proj", "children", "blf-bnt", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 child, got %d", len(issues))
	}
}

func TestDepList_argvAndDecode(t *testing.T) {
	runner := &fakeRunner{output: []byte(depFixture)}
	a := &Adapter{Runner: runner}

	deps, err := a.DepList("blf-bnt.1")
	if err != nil {
		t.Fatalf("DepList: unexpected error: %v", err)
	}
	wantArgs := []string{"dep", "list", "blf-bnt.1", "--json"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].ID != "blf-bnt" || deps[0].DependencyType != "parent-child" {
		t.Errorf("dependency = %+v, want id=blf-bnt type=parent-child", deps[0])
	}
}

func TestCheck_bdNotFound(t *testing.T) {
	runner := &fakeRunner{err: ErrBdNotFound}
	a := &Adapter{Runner: runner}
	if err := a.Check(); !errors.Is(err, ErrBdNotFound) {
		t.Errorf("Check() = %v, want ErrBdNotFound", err)
	}
}

func TestCheck_noDatabase(t *testing.T) {
	runner := &fakeRunner{err: errors.New("no active beads workspace found")}
	a := &Adapter{Runner: runner}
	err := a.Check()
	if !errors.Is(err, ErrNoDatabase) {
		t.Errorf("Check() = %v, want wrapped ErrNoDatabase", err)
	}
}

func TestCheck_ok(t *testing.T) {
	runner := &fakeRunner{output: []byte("/proj/.beads\n")}
	a := &Adapter{Runner: runner, Dir: "/proj"}
	if err := a.Check(); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
	wantArgs := []string{"-C", "/proj", "where"}
	if !reflect.DeepEqual(runner.gotArgs, wantArgs) {
		t.Errorf("argv = %v, want %v", runner.gotArgs, wantArgs)
	}
}
