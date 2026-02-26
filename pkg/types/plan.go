package types

// Plan defines the structure of the YAML/JSON file
type Plan struct {
	Project    string       `yaml:"project" json:"project"`
	Repository string       `yaml:"repository" json:"repository"`
	Milestones []Milestone  `yaml:"milestones" json:"milestones"`
	Epics      []Epic       `yaml:"epics" json:"epics"`
}

// Milestone defines a milestone
type Milestone struct {
	Title       string `yaml:"title" json:"title"`
	DueOn       string `yaml:"due_on" json:"due_on"`
	Description string `yaml:"description" json:"description"`
}

// Epic defines an epic
type Epic struct {
	Title     string   `yaml:"title" json:"title"`
	Body      string   `yaml:"body" json:"body"`
	Milestone string   `yaml:"milestone" json:"milestone"`
	Status    string   `yaml:"status" json:"status"`
	Labels    []string `yaml:"labels" json:"labels"`
	Assignees []string `yaml:"assignees" json:"assignees"`
	Children  []Issue  `yaml:"children" json:"children"`
}

// Issue defines a child issue
type Issue struct {
	Title  string   `yaml:"title" json:"title"`
	Body   string   `yaml:"body" json:"body"`
	Labels []string `yaml:"labels" json:"labels"`
}
