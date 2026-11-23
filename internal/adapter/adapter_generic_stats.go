package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
)

func (a *GenericAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	pageURL := config.Domain + "/index.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(htmlBytes)

	if strings.Contains(html, "login.php") && !strings.Contains(html, "userdetails") && !strings.Contains(html, "userdetail.") {
		return nil, fmt.Errorf("cookie 无效或已过期")
	}
	if strings.Contains(html, "login_act.php") && !strings.Contains(html, "user_info") {
		return nil, fmt.Errorf("cookie 无效或已过期")
	}

	result := &model.UserStatsResult{}
	if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reNexusUsernameSpan.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reNexusUsernameAlt.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reGenericWelcomeUsername.FindStringSubmatch(html); len(m) > 1 {
		result.Username = strings.TrimSpace(m[1])
	}
	if m := reNexusFontUploaded.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reGenericLabelUpload.FindStringSubmatch(html); len(m) > 2 {
		result.UploadBytes = parseSizeString(m[1] + m[2] + "B")
	} else if m := reHDRouteUpload.FindStringSubmatch(html); len(m) > 2 {
		result.UploadBytes = parseSizeString(m[1] + m[2])
	} else if m := reTTGUpload.FindStringSubmatch(html); len(m) > 2 {
		result.UploadBytes = parseSizeString(m[1] + m[2])
	}
	if m := reNexusFontDownloaded.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reGenericLabelDownload.FindStringSubmatch(html); len(m) > 2 {
		result.DownloadBytes = parseSizeString(m[1] + m[2] + "B")
	} else if m := reHDRouteDownload.FindStringSubmatch(html); len(m) > 2 {
		result.DownloadBytes = parseSizeString(m[1] + m[2])
	} else if m := reTTGDownload.FindStringSubmatch(html); len(m) > 2 {
		result.DownloadBytes = parseSizeString(m[1] + m[2])
	}
	if m := reNexusFontRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reNexusLabelRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reHDRouteRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(m[1], 64)
	} else if m := reTTGRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := reGenericBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		if v, err := strconv.ParseFloat(bonusStr, 64); err == nil {
			result.BonusPoints = v
		}
	} else if m := reTTGBonus.FindStringSubmatch(html); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			result.BonusPoints = v
		}
	}
	if m := reGenericSeedingCount.FindStringSubmatch(html); len(m) > 1 {
		result.SeedingCount, _ = strconv.Atoi(strings.TrimSpace(m[1]))
	} else if m := reHDRouteSeeding.FindStringSubmatch(html); len(m) > 1 {
		result.SeedingCount, _ = strconv.Atoi(strings.TrimSpace(m[1]))
	} else if m := reTTGSeeding.FindStringSubmatch(html); len(m) > 1 {
		result.SeedingCount, _ = strconv.Atoi(strings.TrimSpace(m[1]))
	}
	if m := reGenericUserClass.FindStringSubmatch(html); len(m) > 1 {
		if name, ok := starSpaceUserClassMap[m[1]]; ok {
			result.UserClass = name
		} else {
			result.UserClass = "UC" + m[1]
		}
	} else if m := reHDRouteUserClass.FindStringSubmatch(html); len(m) > 1 {
		result.UserClass = m[1]
	}
	if result.UserClass == "" && strings.Contains(config.Domain, "totheglory") {
		if idm := reTTGUserID.FindStringSubmatch(html); len(idm) > 1 {
			result.UserClass = a.fetchTTGUserClass(ctx, config, idm[1])
		}
	}
	// hdroute: extract seeding size from peering-size (first match = seeding)
	if result.SeedingSize == 0 {
		if m := reHDRouteSeedSize.FindStringSubmatch(html); len(m) > 2 {
			result.SeedingSize = parseSizeString(m[1] + m[2])
		}
	}
	if result.Ratio == 0 && result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}
	if result.SeedingSize == 0 {
		a.fetchSeedingSizeFromBonusHour(ctx, config, result)
	}

	return result, nil
}

func (a *GenericAdapter) fetchTTGUserClass(ctx context.Context, config *model.SiteConfig, userID string) string {
	detailURL := config.Domain + "/userdetails.php?id=" + userID
	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	if m := reTTGUserClass.FindStringSubmatch(string(body)); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
