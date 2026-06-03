package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGraphQLLoader_PopulatesReturnType covers the new return-type
// extraction surface introduced for #9 handler emission. The action.md
// emitter doesn't consume these fields, so the golden tests can't pin
// them; assert on the parsed Spec directly.
func TestGraphQLLoader_PopulatesReturnType(t *testing.T) {
	sdl := `
type Query {
  """Get one issue."""
  issue(id: String!): Issue

  """Currently authenticated user."""
  viewer: User

  """A flat scalar return — no selection set needed."""
  serverTime: String!

  """A list of issues — selection should pick up Issue's scalar fields."""
  issues: [Issue!]!
}

type Mutation {
  """Create one issue."""
  issueCreate(input: IssueCreateInput!): IssuePayload!
}

input IssueCreateInput {
  title: String!
}

type Issue {
  id: ID!
  identifier: String!
  title: String!
  description: String
  state: WorkflowState
  team: Team
}

type Team {
  id: ID!
  name: String!
}

type WorkflowState {
  id: ID!
  name: String!
}

type User {
  id: ID!
  name: String!
  email: String
}

type IssuePayload {
  success: Boolean!
  issue: Issue
  lastSyncId: Float!
}
`
	spec := mustParseGraphQL(t, sdl)
	byID := map[string]Operation{}
	for _, op := range spec.Operations {
		byID[op.ID] = op
	}

	cases := []struct {
		opID       string
		wantType   string
		wantScalar bool
		wantFields []ReturnField
	}{
		{
			// Direct object return: selection includes Issue's four
			// scalar fields PLUS one-level recursion into the nested
			// state and team objects (their own scalar fields).
			opID:       "issue",
			wantType:   "Issue",
			wantScalar: false,
			wantFields: []ReturnField{
				{Name: "description"},
				{Name: "id"},
				{Name: "identifier"},
				{Name: "state", Nested: []ReturnField{{Name: "id"}, {Name: "name"}}},
				{Name: "team", Nested: []ReturnField{{Name: "id"}, {Name: "name"}}},
				{Name: "title"},
			},
		},
		{
			// Zero-arg query, flat object return.
			opID:       "viewer",
			wantType:   "User",
			wantScalar: false,
			wantFields: []ReturnField{
				{Name: "email"},
				{Name: "id"},
				{Name: "name"},
			},
		},
		{
			// Scalar return — no selection set.
			opID:       "serverTime",
			wantType:   "String",
			wantScalar: true,
			wantFields: nil,
		},
		{
			// List of objects: unwrap to inner Issue, same fields as
			// the direct-Issue case.
			opID:       "issues",
			wantType:   "Issue",
			wantScalar: false,
			wantFields: []ReturnField{
				{Name: "description"},
				{Name: "id"},
				{Name: "identifier"},
				{Name: "state", Nested: []ReturnField{{Name: "id"}, {Name: "name"}}},
				{Name: "team", Nested: []ReturnField{{Name: "id"}, {Name: "name"}}},
				{Name: "title"},
			},
		},
		{
			// Mutation wrapper: scalars of IssuePayload (lastSyncId,
			// success) PLUS one-level recursion into the nested
			// `issue` field. Crucially, the recursion stops at
			// scalar children of Issue — `state` and `team` (objects
			// at depth 2) are not surfaced, by design.
			opID:       "issueCreate",
			wantType:   "IssuePayload",
			wantScalar: false,
			wantFields: []ReturnField{
				{Name: "issue", Nested: []ReturnField{
					{Name: "description"},
					{Name: "id"},
					{Name: "identifier"},
					{Name: "title"},
				}},
				{Name: "lastSyncId"},
				{Name: "success"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.opID, func(t *testing.T) {
			op, ok := byID[tc.opID]
			if !ok {
				t.Fatalf("op %q not in parsed spec", tc.opID)
			}
			if op.ReturnType != tc.wantType {
				t.Errorf("ReturnType = %q, want %q", op.ReturnType, tc.wantType)
			}
			if op.ReturnTypeIsScalar != tc.wantScalar {
				t.Errorf("ReturnTypeIsScalar = %v, want %v", op.ReturnTypeIsScalar, tc.wantScalar)
			}
			if !equalReturnFields(op.ReturnFields, tc.wantFields) {
				t.Errorf("ReturnFields mismatch:\n  got:  %+v\n  want: %+v", op.ReturnFields, tc.wantFields)
			}
		})
	}
}

func equalReturnFields(a, b []ReturnField) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if !equalReturnFields(a[i].Nested, b[i].Nested) {
			return false
		}
	}
	return true
}

func mustParseGraphQL(t *testing.T, sdl string) Spec {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "schema.graphql")
	if err := os.WriteFile(path, []byte(sdl), 0o600); err != nil {
		t.Fatalf("write sdl: %v", err)
	}
	spec, err := loadGraphQLSpec(path)
	if err != nil {
		t.Fatalf("loadGraphQLSpec: %v", err)
	}
	return spec
}

