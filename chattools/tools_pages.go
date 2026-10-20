package chattools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"
)

func init() {
	Register(newTool(
		"clone_page",
		"Mirror a public login/landing page URL into the page library via the platform AI rewrite pipeline (authorized red-team drill). Runs immediately. Always use this tool when the operator pastes a URL and asks to clone/mirror/入库.",
		RiskWrite,
		schema(map[string]any{
			"url":         map[string]any{"type": "string", "description": "Target page URL to fetch and mirror"},
			"name":        map[string]any{"type": "string", "description": "Optional library page name"},
			"description": map[string]any{"type": "string", "description": "Optional description"},
		}, []string{"url"}),
		"",
		toolClonePage,
	))
	Register(newTool(
		"list_pages",
		"List page-library entries the current user can access (id, name, url, description).",
		RiskRead,
		schema(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "Max rows (default 20, max 100)"},
		}, nil),
		"",
		toolListPages,
	))
	Register(newTool(
		"get_page",
		"Get one page-library entry's metadata. HTML is never returned in full.",
		RiskRead,
		schema(map[string]any{
			"page_id": map[string]any{"type": "integer", "description": "Page library id"},
		}, []string{"page_id"}),
		"",
		toolGetPage,
	))
}

func toolClonePage(ec ExecContext, args map[string]any) Result {
	urlStr := jsonString(args["url"])
	if urlStr == "" {
		return Result{OK: false, Error: "url is required"}
	}
	name := jsonString(args["name"])
	desc := jsonString(args["description"])

	profile, apiKey, active := platform.ActiveAIConfig()
	if !active {
		return Result{OK: false, Error: "No AI model is enabled. Open System → AI Settings."}
	}
	if strings.TrimSpace(apiKey) == "" {
		return Result{OK: false, Error: "Enabled AI profile has no API key."}
	}

	ec.Progress("clone", 10, "Fetching target page")
	rawHTML, finalURL, err := utils.FetchURLHTMLForClone(ec.Context(), urlStr, 30*time.Second)
	if err != nil {
		return Result{OK: false, Error: "Failed to fetch page: " + err.Error()}
	}
	originalHost := ""
	if finalURL != nil {
		originalHost = finalURL.Host
	}
	submitURL := utils.LandingSubmitToken("")
	redirectURL := utils.LandingRedirectToken("")
	aiHTML := utils.StripHeavyBase64DataURIs(rawHTML)
	timeout := utils.AIRewriteTimeout(time.Duration(profile.TimeoutSec)*time.Second, len(aiHTML))
	aiCtx, cancel := context.WithTimeout(ec.Context(), timeout+45*time.Second)
	defer cancel()

	originalURL := urlStr
	if finalURL != nil {
		originalURL = finalURL.String()
	}
	ec.Progress("clone", 45, "Rewriting page HTML with AI")
	rewritten, err := utils.RewriteHTMLWithAIFormat(aiCtx, profile.APIFormat, profile.BaseURL, apiKey, profile.Model, timeout, utils.MirrorRewriteInput{
		RawHTML:      aiHTML,
		SubmitURL:    submitURL,
		RedirectURL:  redirectURL,
		OriginalHost: originalHost,
		OriginalURL:  originalURL,
	})
	if err != nil {
		return Result{OK: false, Error: "AI rewrite failed: " + err.Error()}
	}
	if err := utils.ValidateRewrittenHTML(rewritten, submitURL); err != nil {
		return Result{OK: false, Error: "AI rewrite failed validation: " + err.Error()}
	}
	// Root-relative CSS/JS/images must point at the origin host or they 404 on Spearhook.
	if finalURL != nil {
		rewritten = utils.AbsolutizeHTMLAssets(rewritten, finalURL.String())
	} else {
		rewritten = utils.AbsolutizeHTMLAssets(rewritten, urlStr)
	}

	if name == "" {
		if finalURL != nil && finalURL.Host != "" {
			name = finalURL.Host
		} else {
			name = "Mirrored page"
		}
	}
	pageURL := strings.TrimSpace(urlStr)
	if finalURL != nil {
		pageURL = finalURL.String()
	}
	if desc == "" {
		desc = "Mirrored from " + pageURL
	}

	page := models.PhishingPage{
		Name:        name,
		URL:         pageURL,
		Description: desc,
		Html:        rewritten,
		CreatedBy:   ec.User.ID,
	}
	ec.Progress("clone", 85, "Saving page to library")
	if err := config.GetDB().Create(&page).Error; err != nil {
		return Result{OK: false, Error: "Failed to save mirrored page: " + err.Error()}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("cloned page #%d (%s)", page.ID, page.Name),
		Result: map[string]any{
			"id":          page.ID,
			"name":        page.Name,
			"url":         page.URL,
			"description": page.Description,
			"created_at":  page.CreatedAt,
		},
		Links: []Link{{Label: "页面制作", Href: "/workbench/page-builder"}},
	}
}

func toolListPages(ec ExecContext, args map[string]any) Result {
	limit := 20
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 100 {
		limit = 100
	}
	db := config.GetDB()
	var rows []models.PhishingPage
	if err := platform.ScopeByOwner(db.Model(&models.PhishingPage{}), ec.User).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list pages"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		out = append(out, map[string]any{
			"id":          p.ID,
			"name":        p.Name,
			"url":         p.URL,
			"description": p.Description,
			"created_at":  p.CreatedAt,
		})
	}
	return Result{OK: true, Summary: fmt.Sprintf("%d pages", len(out)), Result: out}
}

func toolGetPage(ec ExecContext, args map[string]any) Result {
	id, ok := jsonInt(args["page_id"])
	if !ok || id <= 0 {
		return Result{OK: false, Error: "page_id is required"}
	}
	db := config.GetDB()
	var page models.PhishingPage
	if err := platform.ScopeByOwner(db, ec.User).First(&page, uint(id)).Error; err != nil {
		return Result{OK: false, Error: "page not found"}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("page #%d %s", page.ID, page.Name),
		Result: map[string]any{
			"id":          page.ID,
			"name":        page.Name,
			"url":         page.URL,
			"description": page.Description,
			"html_bytes":  len(page.Html),
			"html_head":   truncateRunes(page.Html, 400),
			"created_at":  page.CreatedAt,
			"updated_at":  page.UpdatedAt,
		},
		Links: []Link{{Label: "页面制作", Href: "/workbench/page-builder"}},
	}
}
