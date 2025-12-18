package mcp

import "testing"

func TestValidateSQLSyntaxError(t *testing.T) {
	res := validateSQL("SELCT 1")
	if res.IsValid {
		t.Fatalf("expected validation to fail on syntax error")
	}
	found := false
	for _, i := range res.Issues {
		if i.Rule == "syntax_error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected syntax_error issue")
	}
}

func TestValidateSQLSecurityWarning(t *testing.T) {
	res := validateSQL("SELECT * FROM users WHERE name = 'a' OR 1=1")
	found := false
	for _, i := range res.Issues {
		if i.Rule == "tautology" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tautology warning")
	}
}

func TestValidateSQLPass(t *testing.T) {
	res := validateSQL("SELECT id, name FROM users WHERE id = 1")
	if !res.IsValid {
		t.Fatalf("expected validation to pass, got issues: %+v", res.Issues)
	}
}
