package description

import (
	"fmt"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

type Renderer struct {
	format string
}

func NewRenderer(format string) *Renderer {
	return &Renderer{format: format}
}

func (r *Renderer) Render(data *model.DescriptionData, config model.SiteDescConfig) (string, error) {
	if data == nil {
		return "", nil
	}

	format := config.Format
	if format == "" {
		format = r.format
	}
	if format == "" {
		format = "bbcode"
	}

	var sections []string

	if data.Statement != "" {
		sections = append(sections, r.renderSection("声明", data.Statement, format))
	}

	if data.PosterURL != "" {
		sections = append(sections, r.renderPoster(data.PosterURL, format))
	}

	if data.PTGenBody != "" {
		sections = append(sections, data.PTGenBody)
	}

	if data.MediaInfoText != "" {
		mediaInfo := r.FormatMediaInfo(data.MediaInfoText, model.MediaInfoFormat(format))
		if mediaInfo != "" {
			sections = append(sections, r.renderSection("MediaInfo", mediaInfo, format))
		}
	}

	if data.BDInfoText != "" {
		sections = append(sections, r.renderSection("BDInfo", r.wrapCode(data.BDInfoText, format), format))
	}

	if len(data.Screenshots) > 0 {
		screenshots := r.FormatScreenshots(data.Screenshots)
		if screenshots != "" {
			sections = append(sections, r.renderSection("截图", screenshots, format))
		}
	}

	if data.SourceSite != "" {
		note := fmt.Sprintf("转载自 %s", data.SourceSite)
		sections = append(sections, r.renderSection("来源", note, format))
	}

	if config.TemplateOverride != "" {
		return r.applyTemplate(config.TemplateOverride, data, format), nil
	}

	return strings.Join(sections, "\n\n"), nil
}

func (r *Renderer) FormatMediaInfo(rawText string, format model.MediaInfoFormat) string {
	if rawText == "" {
		return ""
	}
	return r.wrapCode(strings.TrimSpace(rawText), string(format))
}

func (r *Renderer) FormatScreenshots(urls []string) string {
	if len(urls) == 0 {
		return ""
	}

	var parts []string
	for _, url := range urls {
		if url == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[img]%s[/img]", url))
	}
	return strings.Join(parts, "\n")
}

func (r *Renderer) Format() string {
	return r.format
}

func (r *Renderer) renderSection(title, content, format string) string {
	switch format {
	case "bbcode":
		return fmt.Sprintf("[b]%s[/b]\n%s", title, content)
	case "markdown":
		return fmt.Sprintf("## %s\n\n%s", title, content)
	case "html":
		return fmt.Sprintf("<h3>%s</h3>\n%s", title, content)
	default:
		return fmt.Sprintf("%s\n%s", title, content)
	}
}

func (r *Renderer) renderPoster(url, format string) string {
	switch format {
	case "bbcode":
		return fmt.Sprintf("[img]%s[/img]", url)
	case "markdown":
		return fmt.Sprintf("![poster](%s)", url)
	case "html":
		return fmt.Sprintf(`<img src="%s" alt="poster" />`, url)
	default:
		return url
	}
}

func (r *Renderer) wrapCode(text, format string) string {
	switch format {
	case "bbcode":
		return fmt.Sprintf("[code]%s[/code]", text)
	case "markdown":
		return fmt.Sprintf("```\n%s\n```", text)
	case "html":
		return fmt.Sprintf("<pre><code>%s</code></pre>", text)
	default:
		return text
	}
}

func (r *Renderer) applyTemplate(template string, data *model.DescriptionData, format string) string {
	result := template
	result = strings.ReplaceAll(result, "{{poster}}", r.renderPoster(data.PosterURL, format))
	result = strings.ReplaceAll(result, "{{statement}}", data.Statement)
	result = strings.ReplaceAll(result, "{{ptgen}}", data.PTGenBody)
	result = strings.ReplaceAll(result, "{{mediainfo}}", r.FormatMediaInfo(data.MediaInfoText, model.MediaInfoFormat(format)))
	result = strings.ReplaceAll(result, "{{screenshots}}", r.FormatScreenshots(data.Screenshots))
	result = strings.ReplaceAll(result, "{{source_site}}", data.SourceSite)
	return result
}
