package tui

import "strings"

// tabComplete returns updated input after Tab: full unique command + space, or
// longest common prefix of all matches, or unchanged if no slash-prefix match.
func tabComplete(input string) string {
	s := strings.TrimSpace(input)
	if !strings.HasPrefix(s, "/") {
		return input
	}
	q := strings.ToLower(s)
	var matches []string
	for _, c := range slashCommands {
		if strings.HasPrefix(strings.ToLower(c), q) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return input
	}
	if len(matches) == 1 {
		return matches[0] + " "
	}
	lcp := longestCommonPrefix(matches)
	if len(lcp) > len(s) {
		return lcp
	}
	return input
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		i := 0
		n := len(prefix)
		if len(s) < n {
			n = len(s)
		}
		for i < n && prefix[i] == s[i] {
			i++
		}
		prefix = prefix[:i]
	}
	return prefix
}
