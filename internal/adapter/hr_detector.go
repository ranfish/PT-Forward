package adapter

import (
	"strings"
)

func detectHRFromHTML(html string) bool {
	return strings.Contains(html, `class="hitandrun"`) ||
		strings.Contains(html, `class='hitandrun'`)
}
