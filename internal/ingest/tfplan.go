package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// TerraformPlan represents a simplified Terraform plan structure
type TerraformPlan struct {
	FormatVersion string `json:"format_version"`
	PlannedValues struct {
		RootModule struct {
			Resources []TFResource `json:"resources"`
		} `json:"root_module"`
	} `json:"planned_values"`
	ResourceChanges []TFResourceChange `json:"resource_changes"`
}

// TFResource represents a Terraform resource
type TFResource struct {
	Address string                 `json:"address"`
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Values  map[string]interface{} `json:"values"`
}

// TFResourceChange represents a resource change in the plan
type TFResourceChange struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	Change  struct {
		Actions []string               `json:"actions"`
		Before  map[string]interface{} `json:"before"`
		After   map[string]interface{} `json:"after"`
	} `json:"change"`
}

// ParseTerraform parses a Terraform plan JSON file
func ParseTerraform(path string) (ParseResult, bool, error) {
	result := ParseResult{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, false, nil // Optional file
		}
		return result, false, err
	}

	var plan TerraformPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return result, false, err
	}

	// Process planned resources (focused on IAM policies)
	for _, resource := range plan.PlannedValues.RootModule.Resources {
		if resource.Type == "aws_iam_policy" {
			if policyStr, ok := resource.Values["policy"].(string); ok {
				parsed := parseTFPolicy(resource.Address, policyStr)
				result.Merge(parsed)
			}
		}
	}

	// Process resource changes to detect permission expansions
	for _, change := range plan.ResourceChanges {
		if change.Type == "aws_iam_policy" && slices.Contains(change.Change.Actions, "update") {
			beforePolicy, _ := change.Change.Before["policy"].(string)
			afterPolicy, _ := change.Change.After["policy"].(string)

			if beforePolicy != "" && afterPolicy != "" {
				// Simple detection: check if wildcard was added
				hadWildcard := strings.Contains(beforePolicy, ":*")
				hasWildcard := strings.Contains(afterPolicy, ":*")

				if !hadWildcard && hasWildcard {
					// Permission expansion detected
					parsed := parseTFPolicy(change.Address+"#expanded", afterPolicy)
					result.Merge(parsed)
				}
			}
		}
	}

	return result, true, nil
}

func parseTFPolicy(address, policyJSON string) ParseResult {
	result := ParseResult{}

	var doc PolicyDocument
	if err := json.Unmarshal([]byte(policyJSON), &doc); err != nil {
		return result
	}

	// Create policy node
	policyID := fmt.Sprintf("tf:%s", address)
	result.Nodes = append(result.Nodes, Node{
		ID:     policyID,
		Kind:   KindPolicy,
		Labels: []string{address, "terraform"},
		Props: map[string]string{
			"address": address,
			"source":  "terraform",
		},
	})

	// Process statements
	for i, stmt := range doc.Statement {
		if stmt.Effect != "Allow" {
			continue
		}

		actions := parseStringOrArray(stmt.Action)
		resources := parseStringOrArray(stmt.Resource)

		for _, action := range actions {
			permID := fmt.Sprintf("%s#stmt%d#%s", policyID, i, action)
			result.Nodes = append(result.Nodes, Node{
				ID:     permID,
				Kind:   KindPerm,
				Labels: []string{action},
				Props: map[string]string{
					"action":   action,
					"wildcard": fmt.Sprintf("%t", strings.Contains(action, "*")),
				},
			})

			result.Edges = append(result.Edges, Edge{
				Src:  policyID,
				Dst:  permID,
				Kind: EdgeAllowsAction,
				Props: map[string]string{
					"statement_index": fmt.Sprintf("%d", i),
				},
			})

			for _, resource := range resources {
				result.Nodes = append(result.Nodes, Node{
					ID:     resource,
					Kind:   KindResource,
					Labels: []string{resource},
					Props: map[string]string{
						"arn": resource,
					},
				})

				result.Edges = append(result.Edges, Edge{
					Src:  permID,
					Dst:  resource,
					Kind: EdgeAppliesTo,
					Props: map[string]string{
						"action": action,
					},
				})
			}
		}
	}

	return result
}
