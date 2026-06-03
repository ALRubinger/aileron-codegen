package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGraphQLLoader_SkipsSubscriptionRootFields covers the design
// decision that GraphQL subscription root fields are not emitted as
// Operations, since the downstream handler emitter dispatches via
// single-shot HTTP POST and subscriptions require a streaming transport
// (graphql-ws over WebSocket in practice). See loadGraphQLSpec's
// docstring for the full rationale.
//
// The test asserts the loader emits Query + Mutation ops but no
// Subscription op, even when the subscription field's name collides
// with a Query or Mutation field. The collisionSuffix "Subscription"
// arm is kept as defensive infra for when streaming transport lands;
// this test will start failing in a useful way if a future change
// re-enables subscription emission without updating the docstring.
func TestGraphQLLoader_SkipsSubscriptionRootFields(t *testing.T) {
	sdl := `
type Query {
  """Read one initiative."""
  initiative(id: String!): Initiative

  """Read the authenticated viewer."""
  viewer: Viewer
}

type Mutation {
  """Create an initiative."""
  initiativeCreate(input: InitiativeCreateInput!): InitiativePayload!
}

type Subscription {
  """Push an event when any initiative changes. Skipped by the loader."""
  initiativeUpdated: Initiative

  """Name collision with Query.viewer — skipped, so no Subscription suffix appears."""
  viewer: Viewer
}

input InitiativeCreateInput {
  name: String!
}

type Initiative {
  id: ID!
  name: String!
}

type Viewer {
  id: ID!
}

type InitiativePayload {
  success: Boolean!
}
`
	spec := mustParseGraphQL(t, sdl)

	wantIDs := map[string]string{
		"initiative":       "QUERY",
		"viewer":           "QUERY",
		"initiativeCreate": "MUTATION",
	}
	if got, want := len(spec.Operations), len(wantIDs); got != want {
		t.Fatalf("operation count = %d, want %d (ids=%v)", got, want, opIDs(spec))
	}
	for _, op := range spec.Operations {
		wantMethod, ok := wantIDs[op.ID]
		if !ok {
			t.Errorf("unexpected operation %q (Method=%q) — Subscription should be skipped", op.ID, op.Method)
			continue
		}
		if op.Method != wantMethod {
			t.Errorf("operation %q Method = %q, want %q", op.ID, op.Method, wantMethod)
		}
	}
	for _, op := range spec.Operations {
		if op.Method == "SUBSCRIPTION" {
			t.Errorf("found SUBSCRIPTION operation %q in spec; loader should have skipped it", op.ID)
		}
		if op.ID == "initiativeUpdated" || op.ID == "viewerSubscription" {
			t.Errorf("found subscription-derived operation %q; loader should have skipped it", op.ID)
		}
	}
}

// TestGraphQLLoader_SubscriptionSkipWritesStderrNotice covers the
// observability half of the skip: the loader writes one line to stderr
// per spec load when subscriptions are dropped, so downstream connector
// generate runs visibly account for the missing fields rather than
// silently shrinking the action surface. The line is matched loosely on
// substring to stay resilient to wording tweaks.
func TestGraphQLLoader_SubscriptionSkipWritesStderrNotice(t *testing.T) {
	sdl := `
type Query {
  viewer: Viewer
}

type Subscription {
  viewerUpdated: Viewer
  initiativeUpdated: Viewer
}

type Viewer {
  id: ID!
}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "schema.graphql")
	if err := os.WriteFile(path, []byte(sdl), 0o600); err != nil {
		t.Fatalf("write sdl: %v", err)
	}
	stderr, restore := captureStderr(t)
	defer restore()
	if _, err := loadGraphQLSpec(path); err != nil {
		t.Fatalf("loadGraphQLSpec: %v", err)
	}
	got := stderr()
	if want := "skipped 2 Subscription root field"; !contains(got, want) {
		t.Errorf("stderr = %q, want substring %q", got, want)
	}
}

// TestGraphQLLoader_NoSubscriptionRootIsSilent covers the negative case:
// a schema without a Subscription root must not emit a stderr line.
// Keeps the observability surface noise-free for the common case.
func TestGraphQLLoader_NoSubscriptionRootIsSilent(t *testing.T) {
	sdl := `
type Query {
  viewer: Viewer
}

type Viewer {
  id: ID!
}
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "schema.graphql")
	if err := os.WriteFile(path, []byte(sdl), 0o600); err != nil {
		t.Fatalf("write sdl: %v", err)
	}
	stderr, restore := captureStderr(t)
	defer restore()
	if _, err := loadGraphQLSpec(path); err != nil {
		t.Fatalf("loadGraphQLSpec: %v", err)
	}
	if got := stderr(); got != "" {
		t.Errorf("stderr = %q, want empty (schema has no Subscription root)", got)
	}
}

// captureStderr redirects os.Stderr to a pipe and returns (read, restore)
// — call read() to get everything written, restore() to put the real
// stderr back. Used to assert on the one-line subscription-skip notice
// without leaking it into the test log.
func captureStderr(t *testing.T) (func() string, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		var out []byte
		for {
			n, err := r.Read(buf)
			if n > 0 {
				out = append(out, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		done <- string(out)
	}()
	read := func() string {
		w.Close()
		os.Stderr = orig
		return <-done
	}
	restore := func() {
		if os.Stderr != orig {
			w.Close()
			os.Stderr = orig
			<-done
		}
	}
	return read, restore
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func opIDs(s Spec) []string {
	out := make([]string, 0, len(s.Operations))
	for _, op := range s.Operations {
		out = append(out, op.ID)
	}
	return out
}
