package nsselector

import "testing"

func TestCompile_EmptyInclude_MatchAllGlob(t *testing.T) {
	s, err := Compile(MatchTypeGlob, nil, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !s.Match("kube-system") || !s.Match("jenkins") {
		t.Fatalf("expected match-all")
	}
}

func TestCompile_EmptyInclude_MatchAllRegexp(t *testing.T) {
	s, err := Compile(MatchTypeRegexp, nil, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !s.Match("kube-system") || !s.Match("jenkins") {
		t.Fatalf("expected match-all")
	}
}

func TestExcludeWins(t *testing.T) {
	s, err := Compile(MatchTypeGlob, []string{"*"}, []string{"client-*"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if s.Match("client-10") {
		t.Fatalf("expected excluded namespace to not match")
	}
	if !s.Match("driver-20") {
		t.Fatalf("expected included namespace to match")
	}
}

func TestCompile_InvalidRegexp(t *testing.T) {
	_, err := Compile(MatchTypeRegexp, []string{"("}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}
