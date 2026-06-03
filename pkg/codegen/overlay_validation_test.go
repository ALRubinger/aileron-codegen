package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestActionEmitter_RejectsUnknownOverlayOperation pins the typo-detection
// contract introduced in #7. Pre-#7 a typo in `operations:` was silent
// (no action emitted for the misspelled op); now it surfaces at emission
// time so the author sees the renamed-or-removed-from-spec slip.
func TestActionEmitter_RejectsUnknownOverlayOperation(t *testing.T) {
	spec := Spec{Operations: []Operation{{ID: "issue", Method: "QUERY"}}}
	overlay := Overlay{
		Connector: ConnectorOverlay{Name: "github://x/y"},
		Operations: map[string]OperationOverlay{
			"issueeeee": {}, // typo
		},
	}
	err := (ActionEmitter{}).Emit(spec, overlay, t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown operation in overlay")
	}
	if !strings.Contains(err.Error(), "issueeeee") {
		t.Errorf("err = %v, want it to name the offending op id", err)
	}
}

// TestActionEmitter_RejectsUnknownExclude does the same for exclude
// entries — typo-protection symmetry.
func TestActionEmitter_RejectsUnknownExclude(t *testing.T) {
	spec := Spec{Operations: []Operation{{ID: "issue", Method: "QUERY"}}}
	overlay := Overlay{
		Connector: ConnectorOverlay{Name: "github://x/y"},
		Exclude:   []string{"issueeeee"}, // typo
	}
	err := (ActionEmitter{}).Emit(spec, overlay, t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown operation in exclude")
	}
	if !strings.Contains(err.Error(), "issueeeee") {
		t.Errorf("err = %v, want it to name the offending op id", err)
	}
}

// TestActionEmitter_ExcludeDropsOp pins the exclude semantics: an op
// listed in exclude does not produce an action.md.
func TestActionEmitter_ExcludeDropsOp(t *testing.T) {
	spec := Spec{Operations: []Operation{
		{ID: "issue", Method: "QUERY"},
		{ID: "viewer", Method: "QUERY"},
	}}
	overlay := Overlay{
		Connector: ConnectorOverlay{Name: "github://x/y"},
		Exclude:   []string{"viewer"},
	}
	out := t.TempDir()
	if err := (ActionEmitter{}).Emit(spec, overlay, out); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "actions", "viewer", "action.md")); !os.IsNotExist(err) {
		t.Errorf("viewer/action.md should not exist (excluded), got stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "actions", "issue", "action.md")); err != nil {
		t.Errorf("issue/action.md should exist (not excluded), got stat err = %v", err)
	}
}

// TestResolveDefaults exercises the kind-based defaults table in
// isolation — keeps the table honest if someone reaches in to add a
// method shortcut later. Asserts on the (idempotent, approvalRequired)
// pair every method should resolve to when overlay is empty.
func TestResolveDefaults(t *testing.T) {
	cases := []struct {
		method            string
		wantIdempotent    bool
		wantApprovalReq   bool
	}{
		// Reads.
		{"QUERY", true, false},
		{"SUBSCRIPTION", true, false},
		{"GET", true, false},
		{"HEAD", true, false},
		// Idempotent write.
		{"PUT", true, true},
		// Non-idempotent writes.
		{"MUTATION", false, true},
		{"POST", false, true},
		{"PATCH", false, true},
		{"DELETE", false, true},
		// Case insensitivity.
		{"query", true, false},
		{"mutation", false, true},
		// Unknown method falls through to non-idempotent + approval
		// required — conservative.
		{"", false, true},
		{"WEBHOOK", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			op := Operation{ID: "x", Method: tc.method}
			if got := resolveIdempotent(op, OperationOverlay{}); got != tc.wantIdempotent {
				t.Errorf("idempotent = %v, want %v", got, tc.wantIdempotent)
			}
			if got := resolveApprovalRequired(op, OperationOverlay{}); got != tc.wantApprovalReq {
				t.Errorf("approvalRequired = %v, want %v", got, tc.wantApprovalReq)
			}
		})
	}
}

// TestResolveIdempotent_OverlayOverridesDefault confirms the pointer-
// based override semantics: an overlay that explicitly sets idempotent
// (either true or false) wins over the kind-based default.
func TestResolveIdempotent_OverlayOverridesDefault(t *testing.T) {
	yes := true
	no := false
	op := Operation{Method: "MUTATION"} // default: false
	if got := resolveIdempotent(op, OperationOverlay{Idempotent: &yes}); got != true {
		t.Errorf("MUTATION + override true: got %v, want true", got)
	}
	op = Operation{Method: "QUERY"} // default: true
	if got := resolveIdempotent(op, OperationOverlay{Idempotent: &no}); got != false {
		t.Errorf("QUERY + override false: got %v, want false", got)
	}
}

// TestResolveCapabilities_DefaultIsOpSnakeCase confirms the
// no-overlay-entry default is a single-entry list of snake_case(opId).
func TestResolveCapabilities_DefaultIsOpSnakeCase(t *testing.T) {
	op := Operation{ID: "issueCreate"}
	caps := resolveCapabilities(op, OperationOverlay{})
	if len(caps) != 1 || caps[0] != "issue_create" {
		t.Errorf("default caps = %v, want [issue_create]", caps)
	}
}
