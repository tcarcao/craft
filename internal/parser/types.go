package parser

type Architecture struct {
	Systems []*System
	Flows   []*Flow
}

type System struct {
	Name     string
	Contexts []*Context
}

type Context struct {
	Name       string
	Aggregates []string
	Components []*Component
	Services   []*Service
	Events     []string
	Relations  []*Relation
}

type Component struct {
	Name string
	Tech *Technology
}

type Service struct {
	Name     string
	Tech     *Technology
	Platform string
}

type Relation struct {
	Type    string // upstream or downstream
	Target  string
	Pattern string // acl, ohs, or conformist
}

type Flow struct {
	Source    string
	Operation string
	Args      []string
	Target    *FlowTarget
}

type FlowTarget struct {
	Context   string
	Operation string
}

type Technology struct {
	Language string
}
