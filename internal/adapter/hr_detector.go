package adapter

import (
	"strings"
)

func detectHRFromHTML(html string) bool {
	if strings.Contains(html, `class="hitandrun"`) ||
		strings.Contains(html, `class='hitandrun'`) {
		return true
	}
	// Image-based HR markers (e.g. TTG /pic/ico_hr.gif, /pic/hit_run.gif).
	lower := strings.ToLower(html)
	return strings.Contains(lower, "ico_hr.gif") ||
		strings.Contains(lower, "hit_run.gif")
}
