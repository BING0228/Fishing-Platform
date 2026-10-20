package chattools

// Playbooks are preset recipes surfaced as chat chips on an empty session.
// The backend keeps them in one place so the prompt, tests, and (optional) API
// stay aligned.
var Playbooks = []Playbook{
	{
		ID:       "clone_deploy",
		TitleZH:  "克隆并部署",
		TitleEN:  "Clone & deploy",
		PromptZH: "克隆 https://example.com/login 到页面库，创建项目并部署到在线 Agent",
		PromptEN: "Clone https://example.com/login into the page library, create a project and deploy to an online Agent",
		ToolHints: []string{
			"clone_page",
			"create_project",
		},
	},
	{
		ID:       "gather_contacts",
		TitleZH:  "搜集联系信息",
		TitleEN:  "Gather contacts",
		PromptZH: "对 example.com 启动信息搜集，找出邮箱和电话",
		PromptEN: "Start info gathering on example.com to find emails and phones",
		ToolHints: []string{
			"start_info_gather",
		},
	},
	{
		ID:       "hook_notify",
		TitleZH:  "绑定推送通道",
		TitleEN:  "Bind notify hook",
		PromptZH: "列出我的机器人，并为一个项目绑定推送通道",
		PromptEN: "List my robots and bind a notify channel to a project",
		ToolHints: []string{
			"list_robots",
			"bind_robot",
		},
	},
}

// PlaybookByID returns one playbook by its stable id.
func PlaybookByID(id string) (Playbook, bool) {
	for _, p := range Playbooks {
		if p.ID == id {
			return p, true
		}
	}
	return Playbook{}, false
}
