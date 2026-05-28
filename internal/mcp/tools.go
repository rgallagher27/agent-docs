package mcp

// tool is the JSON shape one entry takes in the tools/list response.
type tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema schema `json:"inputSchema"`
}

type schema struct {
	Type       string              `json:"type"`
	Properties map[string]property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// contentBlock is one item in a tool-call result's content array.
// MVP only emits "text" blocks; the spec also supports image/resource.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type toolCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// tools is the static tool catalogue advertised on tools/list. Keep
// descriptions concrete — they are the agent's only documentation
// for how to call these correctly.
var tools = []tool{
	{
		Name:        "write_doc",
		Description: "Create or update an HTML doc at a path in a project's ref. Commits the change and pushes to origin. Use this for plans, ADRs, features, and reviews. The committed file should be self-contained HTML with inline CSS (per ADR-002 in this project's docs).",
		InputSchema: schema{
			Type: "object",
			Properties: map[string]property{
				"project": {Type: "string", Description: "Project slug as registered in the agent-docs config."},
				"ref":     {Type: "string", Description: "Branch to commit to (e.g. \"main\", \"feat-foo\"). Must already exist in the project's bare clone."},
				"path":    {Type: "string", Description: "Path relative to project root. Follow the section convention: plans/YYYY-MM-DD-{slug}.html, decisions/NNN-{name}.html, features/{name}.html, reviews/YYYY-MM-DD-{slug}.html."},
				"content": {Type: "string", Description: "Full self-contained HTML. Inline CSS, breadcrumb at top, header, footer — see the project's docs-html/_assets/STYLE_AND_TEMPLATE.md."},
				"message": {Type: "string", Description: "Commit message. Optional; defaults to a generated one based on the path."},
			},
			Required: []string{"project", "ref", "path", "content"},
		},
	},
	{
		Name:        "read_doc",
		Description: "Read an HTML doc's content at a path in a project at a specific ref. Returns the full file contents as text. Useful for reviewing existing docs before editing them.",
		InputSchema: schema{
			Type: "object",
			Properties: map[string]property{
				"project": {Type: "string", Description: "Project slug."},
				"ref":     {Type: "string", Description: "Branch, tag, or commit SHA. Defaults to the project's trunk ref if omitted."},
				"path":    {Type: "string", Description: "Path of the doc relative to the project root."},
			},
			Required: []string{"project", "path"},
		},
	},
	{
		Name:        "list_docs",
		Description: "List entries in a project, optionally scoped to a section directory (plans, features, decisions, reviews) and a ref. Returns one entry per line; directories carry a trailing slash.",
		InputSchema: schema{
			Type: "object",
			Properties: map[string]property{
				"project": {Type: "string", Description: "Project slug."},
				"ref":     {Type: "string", Description: "Branch, tag, or SHA. Defaults to the project's trunk ref if omitted."},
				"section": {Type: "string", Description: "Section name or sub-directory (e.g. \"plans\"). Empty lists the root."},
			},
			Required: []string{"project"},
		},
	},
}
