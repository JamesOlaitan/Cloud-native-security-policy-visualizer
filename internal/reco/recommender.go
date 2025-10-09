package reco

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

// Recommendation represents a least-privilege recommendation
type Recommendation struct {
	PolicyID           string   `json:"policyId"`
	SuggestedActions   []string `json:"suggestedActions"`
	SuggestedResources []string `json:"suggestedResources"`
	PatchJSON          string   `json:"patchJson"`
	Rationale          string   `json:"rationale"`
}

// Recommender generates least-privilege recommendations
type Recommender struct {
	g *graph.Graph
}

// New creates a new recommender
func New(g *graph.Graph) *Recommender {
	return &Recommender{g: g}
}

// Recommend generates a least-privilege recommendation for a wildcard policy
// It analyzes paths from principals with this policy to target resources
func (r *Recommender) Recommend(policyID string, targetID string, tags []string, cap int) (*Recommendation, error) {
	if cap <= 0 {
		cap = 20
	}

	// Get the policy node
	policy, ok := r.g.GetNode(policyID)
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", policyID)
	}

	// Check if policy has wildcards
	if !hasWildcard(policy) {
		return &Recommendation{
			PolicyID:           policyID,
			SuggestedActions:   []string{},
			SuggestedResources: []string{},
			PatchJSON:          "[]",
			Rationale:          "Policy does not contain wildcard permissions",
		}, nil
	}

	// Find principals that have this policy
	principals := r.findPrincipalsWithPolicy(policyID)
	if len(principals) == 0 {
		return nil, fmt.Errorf("no principals found with policy %s", policyID)
	}

	// Collect observed actions and resources from paths
	actions := make(map[string]bool)
	resources := make(map[string]bool)

	// Determine target resources
	var targets []string
	if targetID != "" {
		targets = []string{targetID}
	} else if containsTag(tags, "sensitive") {
		targets = r.findSensitiveResources()
	} else {
		// Use all resources as targets
		targets = r.findAllResources()
	}

	// Analyze paths from each principal to targets
	for _, principalID := range principals {
		for _, targetResID := range targets {
			// Find path
			nodes, edges, err := r.g.ShortestPath(principalID, targetResID, 8)
			if err != nil {
				continue
			}

			// Check if path includes the policy
			includesPolicy := false
			for _, node := range nodes {
				if node.ID == policyID {
					includesPolicy = true
					break
				}
			}

			if !includesPolicy {
				continue
			}

			// Extract actions and resources from path
			for _, edge := range edges {
				if action, ok := edge.Props["action"]; ok {
					actions[action] = true
				}
			}

			// Add the target resource
			resources[targetResID] = true
		}
	}

	// Convert to sorted slices
	suggestedActions := make([]string, 0, len(actions))
	for action := range actions {
		// Skip wildcards in suggestions
		if !isWildcard(action) {
			suggestedActions = append(suggestedActions, action)
		}
	}
	sort.Strings(suggestedActions)

	suggestedResources := make([]string, 0, len(resources))
	for resource := range resources {
		suggestedResources = append(suggestedResources, resource)
	}
	sort.Strings(suggestedResources)

	// Cap the results
	if len(suggestedActions) > cap {
		suggestedActions = suggestedActions[:cap]
	}
	if len(suggestedResources) > cap {
		suggestedResources = suggestedResources[:cap]
	}

	// Generate JSON Patch (RFC 6902)
	patch := r.generateJSONPatch(policy, suggestedActions, suggestedResources)
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	// Build rationale
	rationale := fmt.Sprintf(
		"Policy %s contains wildcard permissions. Based on analysis of %d principal(s) accessing %d resource(s), "+
			"we recommend narrowing to %d specific action(s) and %d resource(s). "+
			"This follows the principle of least privilege by granting only the permissions actually used.",
		truncatePolicyID(policyID),
		len(principals),
		len(targets),
		len(suggestedActions),
		len(suggestedResources),
	)

	return &Recommendation{
		PolicyID:           policyID,
		SuggestedActions:   suggestedActions,
		SuggestedResources: suggestedResources,
		PatchJSON:          string(patchJSON),
		Rationale:          rationale,
	}, nil
}

// findPrincipalsWithPolicy finds all principals that have the given policy
func (r *Recommender) findPrincipalsWithPolicy(policyID string) []string {
	var principals []string

	// Iterate through all edges to find HAS_POLICY edges pointing to this policy
	for _, edge := range r.g.GetEdges() {
		if edge.Dst == policyID && (edge.Kind == "HAS_POLICY" || edge.Kind == "HAS_ROLE") {
			principals = append(principals, edge.Src)
		}
	}

	return principals
}

// findSensitiveResources returns all resources marked as sensitive
func (r *Recommender) findSensitiveResources() []string {
	var resources []string

	for _, node := range r.g.GetNodes() {
		if node.Kind == ingest.RESOURCE {
			if val, ok := node.Props["sensitive"]; ok && val == "true" {
				resources = append(resources, node.ID)
			}
		}
	}

	return resources
}

// findAllResources returns all resource nodes
func (r *Recommender) findAllResources() []string {
	var resources []string

	for _, node := range r.g.GetNodes() {
		if node.Kind == ingest.RESOURCE {
			resources = append(resources, node.ID)
		}
	}

	return resources
}

// generateJSONPatch creates an RFC 6902 JSON Patch
func (r *Recommender) generateJSONPatch(policy ingest.Node, actions, resources []string) []map[string]interface{} {
	var patches []map[string]interface{}

	// For AWS IAM policies, we'd typically patch Statement[*].Action and Statement[*].Resource
	// For K8s RBAC, we'd patch rules[*].verbs and rules[*].resources
	// This is a simplified version that works for both

	if len(actions) > 0 {
		patches = append(patches, map[string]interface{}{
			"op":    "replace",
			"path":  "/Statement/0/Action",
			"value": actions,
		})
	}

	if len(resources) > 0 {
		patches = append(patches, map[string]interface{}{
			"op":    "replace",
			"path":  "/Statement/0/Resource",
			"value": resources,
		})
	}

	return patches
}

// hasWildcard checks if a policy contains wildcard permissions
func hasWildcard(policy ingest.Node) bool {
	// Check in props for wildcard indicators
	for key, val := range policy.Props {
		if key == "action" || key == "actions" || key == "verbs" {
			if isWildcard(val) {
				return true
			}
		}
		if key == "resource" || key == "resources" {
			if isWildcard(val) {
				return true
			}
		}
	}
	return false
}

// isWildcard checks if a value is a wildcard
func isWildcard(val string) bool {
	return val == "*" || strings.HasSuffix(val, ":*") || strings.HasSuffix(val, "/*")
}

// containsTag checks if tags contain a specific tag
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// truncatePolicyID shortens policy ID for display
func truncatePolicyID(id string) string {
	if len(id) <= 60 {
		return id
	}
	return id[:57] + "..."
}
