package policy

import (
	"strings"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

// BuildInput constructs OPA input from a graph snapshot
func BuildInput(g *graph.Graph) map[string]interface{} {
	input := map[string]interface{}{
		"roles":    map[string]interface{}{},
		"policies": map[string]interface{}{},
		"k8s": map[string]interface{}{
			"bindings": map[string]interface{}{},
		},
	}

	nodes := g.GetNodes()
	edges := g.GetEdges()

	// Build roles map
	roles := input["roles"].(map[string]interface{})
	for _, node := range nodes {
		if node.Kind == ingest.KindPrincipal && contains(node.Labels, "aws-role") {
			roleData := map[string]interface{}{
				"arn":  node.ID,
				"name": node.Props["name"],
				"trust": map[string]interface{}{
					"cross_account": false,
				},
			}

			// Check for cross-account trust
			for _, edge := range edges {
				if edge.Src == node.ID && edge.Kind == ingest.EdgeTrustsCrossAccount {
					roleData["trust"] = map[string]interface{}{
						"cross_account": true,
					}
					break
				}
			}

			roles[node.ID] = roleData
		}
	}

	// Build policies map
	policies := input["policies"].(map[string]interface{})
	for _, node := range nodes {
		if node.Kind == ingest.KindPolicy {
			hasWildcard := false

			// Check for wildcard actions in connected permissions
			for _, edge := range edges {
				if edge.Src == node.ID && edge.Kind == ingest.EdgeAllowsAction {
					// Find the permission node
					for _, permNode := range nodes {
						if permNode.ID == edge.Dst && permNode.Kind == ingest.KindPerm {
							if permNode.Props["wildcard"] == "true" {
								hasWildcard = true
								break
							}
							if action, ok := permNode.Props["action"]; ok && strings.Contains(action, "*") {
								hasWildcard = true
								break
							}
						}
					}
					if hasWildcard {
						break
					}
				}
			}

			policyData := map[string]interface{}{
				"id":                      node.ID,
				"name":                    node.Props["name"],
				"action_matches_wildcard": hasWildcard,
			}

			policies[node.ID] = policyData
		}
	}

	// Build K8s bindings map
	bindings := input["k8s"].(map[string]interface{})["bindings"].(map[string]interface{})
	bindingsMap := make(map[string]bool)

	for _, edge := range edges {
		if edge.Kind == ingest.EdgeBindsTo {
			// Find the role
			for _, node := range nodes {
				if node.ID == edge.Src && node.Kind == ingest.KindRole {
					bindingName := edge.Props["binding"]
					if bindingName == "" {
						bindingName = node.Props["name"]
					}

					if _, exists := bindingsMap[bindingName]; !exists {
						isClusterAdmin := node.Props["cluster_admin"] == "true" || node.Props["name"] == "cluster-admin"

						bindingData := map[string]interface{}{
							"name":          bindingName,
							"cluster_admin": isClusterAdmin,
						}

						bindings[bindingName] = bindingData
						bindingsMap[bindingName] = true
					}
					break
				}
			}
		}
	}

	return input
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
