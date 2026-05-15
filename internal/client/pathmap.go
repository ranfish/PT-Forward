package client

import (
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

func MapPath(sourcePath string, mappings []model.SharedPathMapping) string {
	for _, mapping := range mappings {
		if strings.HasPrefix(sourcePath, mapping.SourcePath) {
			return mapping.ReseedPath + strings.TrimPrefix(sourcePath, mapping.SourcePath)
		}
	}
	return sourcePath
}
