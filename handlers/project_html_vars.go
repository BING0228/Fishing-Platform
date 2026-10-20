package handlers

import (
	"strconv"
	"strings"

	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"
)

// projectHTMLVars builds placeholder values for a project's landing HTML.
func projectHTMLVars(project *models.Project) utils.ProjectHTMLVars {
	return platform.ProjectHTMLVars(project)
}

func parseOptionalUintString(raw string) *uint {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return nil
	}
	u := uint(v)
	return &u
}
