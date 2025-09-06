package visualizer

// C4System represents a software system in C4
type C4System struct {
	Name        string
	Description string
	Containers  []string
	IsExternal  bool
}

// C4Container represents a container within a system
type C4Container struct {
	Name        string
	System      string
	Technology  string
	Description string
	Domains     []string
	DataStores  []string
}

// C4Relation represents a relationship between C4 elements
type C4Relation struct {
	From        string
	To          string
	Description string
	Technology  string
	Type        string // "uses", "reads", "writes", "triggers"
}

// C4DiagramType determines the level of C4 diagram to generate
type C4DiagramType string

const (
	C4Context    C4DiagramType = "context"   // System Context level
	C4Containers C4DiagramType = "container" // Container level
	C4Components C4DiagramType = "component" // Component level (domains as components)
)
