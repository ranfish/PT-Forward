package adapter

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
)

func isSiteTotalMatch(search string, m []string) bool {
	if len(m) == 0 {
		return false
	}
	idx := strings.Index(search, m[0])
	if idx < 0 {
		return false
	}
	prefix := search[:idx]
	return strings.HasSuffix(prefix, "总")
}

var reAnonSuffix = regexp.MustCompile(`\s*\(匿名[^)]*\)\s*$`)

func cleanUsername(name string) string {
	name = strings.TrimSpace(name)
	name = reAnonSuffix.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

func isValidUserClass(class string) string {
	class = strings.TrimSpace(class)
	if class == "" {
		return ""
	}
	if _, err := strconv.Atoi(class); err == nil {
		return ""
	}
	if strings.HasPrefix(class, "UID") {
		return ""
	}
	return class
}

func (a *NexusPHPAdapter) resolveUIDFromSelfProfile(ctx context.Context, config *model.SiteConfig) string {
	pageURL := config.Domain + "/userdetails"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return ""
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return ""
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return ""
	}

	html := string(body)
	if m := reUserIDLoose.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := reIDField.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

func (a *NexusPHPAdapter) fetchSeedingSizeFromBonus(ctx context.Context, config *model.SiteConfig, result *model.UserStatsResult) {
	pageURL := config.Domain + "/mybonus.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return
	}

	html := string(body)
	if m := reBonusSeedingSize.FindStringSubmatch(html); len(m) > 1 {
		if sz := parseSizeString(m[1]); sz > 0 {
			result.SeedingSize = sz
		}
	}
}
