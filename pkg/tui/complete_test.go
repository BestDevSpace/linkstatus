package tui

import "testing"

func TestLongestCommonPrefix(t *testing.T) {
	if got := longestCommonPrefix([]string{"/service-install", "/service-remove", "/service-status"}); got != "/service-" {
		t.Fatalf("got %q", got)
	}
	if got := longestCommonPrefix([]string{"/stats"}); got != "/stats" {
		t.Fatalf("got %q", got)
	}
}

func TestTabComplete(t *testing.T) {
	if got := tabComplete("/ser"); got != "/service-" {
		t.Fatalf("got %q", got)
	}
	if got := tabComplete("/stats"); got != "/stats " {
		t.Fatalf("got %q", got)
	}
}
