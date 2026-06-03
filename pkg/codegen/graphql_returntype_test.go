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
		opID          string
		wantType      string
		wantScalar    bool
		wantFields    []string
	}{
		{
			// Direct object return: selection set should include
			// scalar fields of Issue (id, identifier, title,
			// description) but not `state` or `team` which are
			// themselves objects.
			opID:       "issue",
			wantType:   "Issue",
			wantScalar: false,
			wantFields: []string{"description", "id", "identifier", "title"},
		},
		{
			opID:       "viewer",
			wantType:   "User",
			wantScalar: false,
			wantFields: []string{"email", "id", "name"},
		},
		{
			// Scalar return — no selection set, no scalar-fields list.
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
			wantFields: []string{"description", "id", "identifier", "title"},
		},
		{
			// Mutation returning a wrapper object: scalar fields of
			// IssuePayload, not of the nested Issue.
			opID:       "issueCreate",
			wantType:   "IssuePayload",
			wantScalar: false,
			wantFields: []string{"lastSyncId", "success"},
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
			if !equalStrings(op.ReturnScalarFields, tc.wantFields) {
				t.Errorf("ReturnScalarFields = %v, want %v", op.ReturnScalarFields, tc.wantFields)
			}
		})
	}
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

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
