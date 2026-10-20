package chattools

import (
	"fmt"
	"strings"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
)

func init() {
	Register(newTool(
		"start_info_gather",
		"Start an asynchronous AI web-search info-gathering job for a target (emails/phones). Use get_info_gather to poll status.",
		RiskWrite,
		schema(map[string]any{
			"target":           map[string]any{"type": "string", "description": "Domain, company, or person to research"},
			"notes":            map[string]any{"type": "string", "description": "Optional extra instructions"},
			"include_x_search": map[string]any{"type": "boolean", "description": "Also search X/Twitter"},
		}, []string{"target"}),
		"",
		toolStartInfoGather,
	))
	Register(newTool(
		"get_info_gather",
		"Get one info-gathering job's status, progress, counts, and summary.",
		RiskRead,
		schema(map[string]any{
			"job_id": map[string]any{"type": "integer", "description": "Job id returned by start_info_gather"},
		}, []string{"job_id"}),
		"",
		toolGetInfoGather,
	))
	Register(newTool(
		"list_info_findings",
		"List up to 100 structured findings (email/phone) for an info-gathering job.",
		RiskRead,
		schema(map[string]any{
			"job_id": map[string]any{"type": "integer", "description": "Job id"},
			"kind":   map[string]any{"type": "string", "enum": []string{"email", "phone"}},
		}, []string{"job_id"}),
		"",
		toolListInfoFindings,
	))
	Register(newTool(
		"list_info_gather_jobs",
		"List recent info-gathering jobs with status and finding counts.",
		RiskRead,
		schema(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "Max rows (default 20, max 100)"},
		}, nil),
		"",
		toolListInfoGatherJobs,
	))
}

func toolStartInfoGather(ec ExecContext, args map[string]any) Result {
	target := jsonString(args["target"])
	if target == "" {
		return Result{OK: false, Error: "target is required"}
	}
	if len(target) > 500 {
		return Result{OK: false, Error: "target is too long"}
	}
	profile, apiKey, active := platform.ActiveAIConfig()
	if !active {
		return Result{OK: false, Error: "No AI model is enabled. Configure one in System → AI Settings"}
	}
	if strings.TrimSpace(apiKey) == "" {
		return Result{OK: false, Error: "Enabled AI profile has no API key"}
	}

	db := config.GetDB()
	job := models.InfoGatherJob{
		Target:         target,
		Notes:          jsonString(args["notes"]),
		IncludeXSearch: jsonBool(args["include_x_search"], false),
		Status:         models.InfoGatherJobPending,
		Model:          profile.Model,
		CreatedBy:      ec.User.ID,
		ChatSessionID:  ec.SessionID,
	}
	if err := db.Create(&job).Error; err != nil {
		return Result{OK: false, Error: "Failed to create job"}
	}
	platform.StartInfoGatherJob(job.ID)
	ec.Progress("gather", 5, "Info gathering queued")

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("started info gather job #%d for %s", job.ID, target),
		Result: map[string]any{
			"id":     job.ID,
			"target": job.Target,
			"status": job.Status,
			"model":  job.Model,
		},
		Links: []Link{{Label: "信息搜集", Href: "/workbench/info-gathering"}},
	}
}

func toolGetInfoGather(ec ExecContext, args map[string]any) Result {
	job, res := loadGatherJob(ec, args)
	if !res.OK {
		return res
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("job #%d %s (%d findings)", job.ID, job.Status, job.FindingCount),
		Result: map[string]any{
			"id":               job.ID,
			"target":           job.Target,
			"status":           job.Status,
			"progress_stage":   job.ProgressStage,
			"progress_percent": job.ProgressPercent,
			"progress_message": job.ProgressMessage,
			"email_count":      job.EmailCount,
			"phone_count":      job.PhoneCount,
			"finding_count":    job.FindingCount,
			"summary_notes":    job.SummaryNotes,
			"error_message":    job.ErrorMessage,
			"started_at":       job.StartedAt,
			"finished_at":      job.FinishedAt,
		},
		Links: []Link{{Label: "信息搜集", Href: "/workbench/info-gathering"}},
	}
}

func toolListInfoFindings(ec ExecContext, args map[string]any) Result {
	job, res := loadGatherJob(ec, args)
	if !res.OK {
		return res
	}
	kind := strings.ToLower(jsonString(args["kind"]))
	db := config.GetDB()
	q := db.Where("job_id = ?", job.ID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var rows []models.InfoGatherFinding
	if err := q.Order("id ASC").Limit(100).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list findings"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		out = append(out, map[string]any{
			"id":         f.ID,
			"kind":       f.Kind,
			"value":      f.Value,
			"label":      f.Label,
			"source_url": f.SourceURL,
			"snippet":    truncateRunes(f.Snippet, 240),
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d findings for job #%d", len(out), job.ID),
		Result:  out,
		Links:   []Link{{Label: "信息搜集", Href: "/workbench/info-gathering"}},
	}
}

func loadGatherJob(ec ExecContext, args map[string]any) (models.InfoGatherJob, Result) {
	id, ok := jsonInt(args["job_id"])
	if !ok || id <= 0 {
		return models.InfoGatherJob{}, Result{OK: false, Error: "job_id is required"}
	}
	db := config.GetDB()
	var job models.InfoGatherJob
	if err := db.First(&job, uint(id)).Error; err != nil {
		return models.InfoGatherJob{}, Result{OK: false, Error: "job not found"}
	}
	if !platform.CanAccessOwned(ec.User, job.CreatedBy) {
		return models.InfoGatherJob{}, Result{OK: false, Error: "access denied"}
	}
	return job, Result{OK: true}
}

func toolListInfoGatherJobs(ec ExecContext, args map[string]any) Result {
	limit := 20
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 100 {
		limit = 100
	}
	db := config.GetDB()
	var rows []models.InfoGatherJob
	if err := platform.ScopeByOwner(db.Model(&models.InfoGatherJob{}), ec.User).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list info gather jobs"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, job := range rows {
		out = append(out, map[string]any{
			"id":               job.ID,
			"target":           job.Target,
			"status":           job.Status,
			"email_count":      job.EmailCount,
			"phone_count":      job.PhoneCount,
			"finding_count":    job.FindingCount,
			"progress_percent": job.ProgressPercent,
			"created_at":       job.CreatedAt,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d info gather jobs", len(out)),
		Result:  out,
		Links:   []Link{{Label: "信息搜集", Href: "/workbench/info-gathering"}},
	}
}
