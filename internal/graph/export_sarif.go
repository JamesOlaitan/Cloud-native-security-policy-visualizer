package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

// SARIF represents a SARIF v2.1.0 document
type SARIF struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun represents a run in SARIF
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool represents the tool information
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver represents the tool driver
type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []SARIFRule `json:"rules"`
}

// SARIFRule represents a rule
type SARIFRule struct {
	ID               string           `json:"id"`
	ShortDescription SARIFDescription `json:"shortDescription"`
	FullDescription  SARIFDescription `json:"fullDescription,omitempty"`
	Help             SARIFDescription `json:"help,omitempty"`
}

// SARIFDescription represents a text description
type SARIFDescription struct {
	Text string `json:"text"`
}

// SARIFResult represents a finding
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	RuleIndex int             `json:"ruleIndex"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

// SARIFMessage represents a result message
type SARIFMessage struct {
	Text string `json:"text"`
}

// SARIFLocation represents a location
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

// SARIFPhysicalLocation represents a physical location
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           SARIFRegion           `json:"region,omitempty"`
}

// SARIFArtifactLocation represents an artifact location
type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

// SARIFRegion represents a region in a file
type SARIFRegion struct {
	StartLine int `json:"startLine,omitempty"`
}

// ExportSARIFAttackPath exports an attack path as SARIF v2.1.0
func ExportSARIFAttackPath(fromID, toID string, nodes []ingest.Node, edges []ingest.Edge) (string, error) {
	if len(nodes) == 0 {
		return "", fmt.Errorf("no nodes in path")
	}

	// Build rules (one per edge kind)
	rulesMap := make(map[string]int)
	var rules []SARIFRule
	ruleIndex := 0

	for _, edge := range edges {
		if _, exists := rulesMap[edge.Kind]; !exists {
			rulesMap[edge.Kind] = ruleIndex
			rules = append(rules, SARIFRule{
				ID: fmt.Sprintf("attack-path/%s", edge.Kind),
				ShortDescription: SARIFDescription{
					Text: fmt.Sprintf("Attack path edge: %s", edge.Kind),
				},
				FullDescription: SARIFDescription{
					Text: fmt.Sprintf("This edge represents a %s relationship in the access graph that can be exploited in an attack path", edge.Kind),
				},
				Help: SARIFDescription{
					Text: "Review and restrict permissions to prevent unauthorized access along this path",
				},
			})
			ruleIndex++
		}
	}

	// Build results (one per hop)
	var results []SARIFResult

	for i := 0; i < len(nodes)-1; i++ {
		fromNode := nodes[i]
		toNode := nodes[i+1]
		edge := edges[i]

		// Determine severity
		level := "warning"
		if isCriticalEdge(edge) {
			level = "error"
		}

		// Build message
		message := fmt.Sprintf("Step %d: %s (%s) → %s (%s) via %s",
			i+1,
			truncateID(fromNode.ID),
			fromNode.Kind,
			truncateID(toNode.ID),
			toNode.Kind,
			edge.Kind,
		)

		if action, ok := edge.Props["action"]; ok {
			message += fmt.Sprintf(" [Action: %s]", action)
		}

		results = append(results, SARIFResult{
			RuleID:    fmt.Sprintf("attack-path/%s", edge.Kind),
			RuleIndex: rulesMap[edge.Kind],
			Level:     level,
			Message: SARIFMessage{
				Text: message,
			},
			Locations: []SARIFLocation{
				{
					PhysicalLocation: SARIFPhysicalLocation{
						ArtifactLocation: SARIFArtifactLocation{
							URI: generateStableURI(fromNode.ID, toNode.ID),
						},
						Region: SARIFRegion{
							StartLine: i + 1,
						},
					},
				},
			},
		})
	}

	sarif := SARIF{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:           "AccessGraph",
						Version:        "1.1.0",
						InformationURI: "https://github.com/jamesolaitan/accessgraph",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}

	output, err := json.MarshalIndent(sarif, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling SARIF: %w", err)
	}

	return string(output), nil
}

// isCriticalEdge determines if an edge represents a critical security issue
func isCriticalEdge(edge ingest.Edge) bool {
	// Cross-account access is critical
	if val, ok := edge.Props["cross_account"]; ok && val == "true" {
		return true
	}

	// Wildcard permissions are critical
	if val, ok := edge.Props["action"]; ok {
		if val == "*" || (len(val) > 2 && val[len(val)-2:] == ":*") {
			return true
		}
	}

	return false
}

// generateStableURI creates a deterministic URI for a node pair
func generateStableURI(fromID, toID string) string {
	// Use hash for stable, short URIs
	h := sha256.New()
	h.Write([]byte(fromID + "->" + toID))
	hash := hex.EncodeToString(h.Sum(nil))[:16]

	return fmt.Sprintf("accessgraph://path/%s", hash)
}
