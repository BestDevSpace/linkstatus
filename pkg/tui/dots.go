package tui

import (
	"fmt"
	"strings"
	"time"
)

const (
	dotsPerPanel = 60
	dotsPerRow   = 30
	dotRune      = '●' // single-column dot; falls back styling in terminal
)

// termui color tags: reddish → green by rating 1–5 (see termui paragraph examples).
func fgTagForRating(r float64) string {
	switch {
	case r < 1.5:
		return "fg:red"
	case r < 2.5:
		return "fg:magenta"
	case r < 3.5:
		return "fg:yellow"
	case r < 4.25:
		return "fg:cyan"
	default:
		return "fg:green"
	}
}

func fgTagEmpty() string {
	return "fg:blue"
}

// formatDotRows builds a 2×30 block of colored dots (termui markup). Older time → left, newer → right.
func formatDotRows(ratings []*float64) string {
	if len(ratings) != dotsPerPanel {
		return ""
	}
	var b strings.Builder
	ch := string(dotRune)
	line := func(start, end int) {
		for i := start; i < end; i++ {
			r := ratings[i]
			if r == nil {
				fmt.Fprintf(&b, "[·](%s)", fgTagEmpty())
			} else {
				fmt.Fprintf(&b, "[%s](%s)", ch, fgTagForRating(*r))
			}
		}
	}
	line(0, dotsPerRow)
	b.WriteByte('\n')
	line(dotsPerRow, dotsPerPanel)
	return b.String()
}

func bucketLabel(window time.Duration) string {
	mins := window.Minutes() / float64(dotsPerPanel)
	if mins < 1 {
		return fmt.Sprintf("%.1f min", mins)
	}
	return fmt.Sprintf("%.0f min", mins)
}

// DotLegend is a plain line above the dot grid (not color-tagged).
func DotLegend(window time.Duration) string {
	return fmt.Sprintf("%s per dot · 60 dots · older→newer\n", bucketLabel(window))
}
