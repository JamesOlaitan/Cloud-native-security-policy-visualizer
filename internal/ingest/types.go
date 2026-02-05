package ingest

// Kind represents the type of graph entity
type Kind string

const (
	KindPrincipal Kind = "PRINCIPAL" // AWS Role/User, K8s ServiceAccount
	KindRole      Kind = "ROLE"
	KindPolicy    Kind = "POLICY"
	KindPerm      Kind = "PERMISSION"
	KindResource  Kind = "RESOURCE"
	KindNS        Kind = "NAMESPACE"
	KindAccount   Kind = "ACCOUNT"
)

// Node represents a graph node
type Node struct {
	ID     string            `json:"id"`
	Kind   Kind              `json:"kind"`
	Labels []string          `json:"labels"`
	Props  map[string]string `json:"props"`
}

// Edge represents a graph edge
type Edge struct {
	Src   string            `json:"src"`
	Dst   string            `json:"dst"`
	Kind  string            `json:"kind"`
	Props map[string]string `json:"props"`
}

// EdgeKind constants
const (
	EdgeAssumesRole        = "ASSUMES_ROLE"
	EdgeTrustsCrossAccount = "TRUSTS_CROSS_ACCOUNT"
	EdgeAttachedPolicy     = "ATTACHED_POLICY"
	EdgeAllowsAction       = "ALLOWS_ACTION"
	EdgeAppliesTo          = "APPLIES_TO"
	EdgeBindsTo            = "BINDS_TO"
	EdgeInNamespace        = "IN_NAMESPACE"
)

// ParseResult holds parsed nodes and edges
type ParseResult struct {
	Nodes []Node
	Edges []Edge
}

// Merge combines two parse results
func (pr *ParseResult) Merge(other ParseResult) {
	pr.Nodes = append(pr.Nodes, other.Nodes...)
	pr.Edges = append(pr.Edges, other.Edges...)
}

// Key returns a unique string key for edge comparison in diffs.
// Format: "src|dst|kind"
func (e Edge) Key() string {
	return e.Src + "|" + e.Dst + "|" + e.Kind
}
