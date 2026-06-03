package codegen

import "testing"

// TestGraphQLLoader_NameCollisionSuffix covers #17: when the same field
// name appears on both `type Query` and `type Mutation`, the loader must
// keep both operations addressable downstream by suffixing the
// Mutation's Operation.ID. The Query keeps the bare name (queries
// dominate read traffic — short names matter). The raw GraphQL field
// name lives on Operation.FieldName so the emitted query string still
// invokes the schema's actual field.
func TestGraphQLLoader_NameCollisionSuffix(t *testing.T) {
	sdl := `
type Query {
  """Fetch one initiative update."""
  initiativeUpdate(id: String!): InitiativeUpdate

  """Fetch one issue — has no mutation collision."""
  issue(id: String!): Issue
}

type Mutation {
  """Create an initiative update — collides with Query.initiativeUpdate."""
  initiativeUpdate(input: InitiativeUpdateInput!): InitiativeUpdatePayload!

  """Create an issue — no collision with Query.issue (different field name)."""
  issueCreate(input: IssueCreateInput!): IssuePayload!
}

input InitiativeUpdateInput {
  body: String!
}

input IssueCreateInput {
  title: String!
}

type InitiativeUpdate {
  id: ID!
  body: String!
}

type Issue {
  id: ID!
  title: String!
}

type InitiativeUpdatePayload {
  success: Boolean!
}

type IssuePayload {
  success: Boolean!
}
`
	spec := mustParseGraphQL(t, sdl)
	byID := map[string]Operation{}
	for _, op := range spec.Operations {
		byID[op.ID] = op
	}

	wantIDs := []string{
		"initiativeUpdate",         // Query side keeps the bare name
		"initiativeUpdateMutation", // Mutation side gets the suffix
		"issue",                    // no collision — bare name on Query
		"issueCreate",              // no collision — bare name on Mutation
	}
	for _, id := range wantIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("expected operation with ID %q, got IDs %v", id, idsOf(spec))
		}
	}
	if len(spec.Operations) != len(wantIDs) {
		t.Errorf("unexpected op count: got %d (%v), want %d (%v)",
			len(spec.Operations), idsOf(spec), len(wantIDs), wantIDs)
	}

	// The two collided ops must keep distinct Methods and the suffixed
	// one must still report the raw GraphQL field name in FieldName so
	// the handler emitter writes a valid query against the schema.
	query := byID["initiativeUpdate"]
	if query.Method != "QUERY" {
		t.Errorf("initiativeUpdate.Method = %q, want QUERY", query.Method)
	}
	if query.FieldName != "initiativeUpdate" {
		t.Errorf("initiativeUpdate.FieldName = %q, want %q", query.FieldName, "initiativeUpdate")
	}

	mutation := byID["initiativeUpdateMutation"]
	if mutation.Method != "MUTATION" {
		t.Errorf("initiativeUpdateMutation.Method = %q, want MUTATION", mutation.Method)
	}
	if mutation.FieldName != "initiativeUpdate" {
		t.Errorf("initiativeUpdateMutation.FieldName = %q, want %q (raw schema field)",
			mutation.FieldName, "initiativeUpdate")
	}

	// Non-colliding ops keep FieldName == ID — invariant the handler
	// emitter's fallback relies on.
	for _, id := range []string{"issue", "issueCreate"} {
		op := byID[id]
		if op.FieldName != id {
			t.Errorf("%s.FieldName = %q, want %q", id, op.FieldName, id)
		}
	}
}

func idsOf(s Spec) []string {
	out := make([]string, 0, len(s.Operations))
	for _, op := range s.Operations {
		out = append(out, op.ID)
	}
	return out
}
