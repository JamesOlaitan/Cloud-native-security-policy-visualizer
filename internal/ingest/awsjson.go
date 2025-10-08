package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AWSRole represents an AWS IAM role
type AWSRole struct {
	RoleName                 string          `json:"RoleName"`
	Arn                      string          `json:"Arn"`
	AssumeRolePolicyDocument json.RawMessage `json:"AssumeRolePolicyDocument"`
}

// AWSPolicy represents an AWS IAM policy
type AWSPolicy struct {
	PolicyName    string `json:"PolicyName"`
	Arn           string `json:"Arn"`
	PolicyVersion struct {
		Document PolicyDocument `json:"Document"`
	} `json:"PolicyVersion"`
}

// PolicyDocument represents an IAM policy document
type PolicyDocument struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

// Statement represents a policy statement
type Statement struct {
	Effect    string          `json:"Effect"`
	Action    json.RawMessage `json:"Action"`
	Resource  json.RawMessage `json:"Resource"`
	Principal json.RawMessage `json:"Principal,omitempty"`
}

// AWSAttachment represents role-to-policy attachments
type AWSAttachment struct {
	RoleName         string `json:"RoleName"`
	AttachedPolicies []struct {
		PolicyName string `json:"PolicyName"`
		PolicyArn  string `json:"PolicyArn"`
	} `json:"AttachedPolicies"`
}

var accountIDPattern = regexp.MustCompile(`:(\d{12}):`)

// ParseAWS parses AWS IAM JSON files from a directory
func ParseAWS(dirPath string) (ParseResult, error) {
	result := ParseResult{
		Nodes: []Node{},
		Edges: []Edge{},
	}

	// Parse roles
	rolesPath := filepath.Join(dirPath, "roles.json")
	roles, err := parseRoles(rolesPath)
	if err != nil {
		return result, fmt.Errorf("parsing roles: %w", err)
	}
	result.Merge(roles)

	// Parse policies
	policiesPath := filepath.Join(dirPath, "policies.json")
	policies, err := parsePolicies(policiesPath)
	if err != nil {
		return result, fmt.Errorf("parsing policies: %w", err)
	}
	result.Merge(policies)

	// Parse attachments
	attachmentsPath := filepath.Join(dirPath, "attachments.json")
	attachments, err := parseAttachments(attachmentsPath)
	if err != nil {
		return result, fmt.Errorf("parsing attachments: %w", err)
	}
	result.Merge(attachments)

	return result, nil
}

func parseRoles(path string) (ParseResult, error) {
	result := ParseResult{}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	var roles []AWSRole
	if err := json.Unmarshal(data, &roles); err != nil {
		return result, err
	}

	accountNodes := make(map[string]bool)

	for _, role := range roles {
		// Create role node
		result.Nodes = append(result.Nodes, Node{
			ID:     role.Arn,
			Kind:   KindPrincipal,
			Labels: []string{role.RoleName, "aws-role"},
			Props: map[string]string{
				"name": role.RoleName,
				"arn":  role.Arn,
			},
		})

		// Parse trust policy
		var trustDoc PolicyDocument
		if err := json.Unmarshal(role.AssumeRolePolicyDocument, &trustDoc); err != nil {
			continue
		}

		for _, stmt := range trustDoc.Statement {
			if stmt.Effect != "Allow" {
				continue
			}

			var principal map[string]interface{}
			if err := json.Unmarshal(stmt.Principal, &principal); err != nil {
				continue
			}

			if awsPrincipal, ok := principal["AWS"]; ok {
				principals := []string{}
				switch v := awsPrincipal.(type) {
				case string:
					principals = append(principals, v)
				case []interface{}:
					for _, p := range v {
						if s, ok := p.(string); ok {
							principals = append(principals, s)
						}
					}
				}

				for _, p := range principals {
					// Extract account ID from principal
					matches := accountIDPattern.FindStringSubmatch(p)
					if len(matches) > 1 {
						accountID := matches[1]
						
						// Check if it's cross-account
						roleAccountMatches := accountIDPattern.FindStringSubmatch(role.Arn)
						if len(roleAccountMatches) > 1 && roleAccountMatches[1] != accountID {
							// Create account node if not exists
							accountArn := fmt.Sprintf("arn:aws:iam::%s:root", accountID)
							if !accountNodes[accountArn] {
								result.Nodes = append(result.Nodes, Node{
									ID:     accountArn,
									Kind:   KindAccount,
									Labels: []string{accountID, "aws-account"},
									Props: map[string]string{
										"account_id": accountID,
									},
								})
								accountNodes[accountArn] = true
							}

							// Create TRUSTS_CROSS_ACCOUNT edge
							result.Edges = append(result.Edges, Edge{
								Src:  role.Arn,
								Dst:  accountArn,
								Kind: EdgeTrustsCrossAccount,
								Props: map[string]string{
									"principal": p,
								},
							})
						}

						// Create ASSUMES_ROLE edge
						result.Edges = append(result.Edges, Edge{
							Src:  p,
							Dst:  role.Arn,
							Kind: EdgeAssumesRole,
							Props: map[string]string{
								"action": "sts:AssumeRole",
							},
						})
					}
				}
			}
		}
	}

	return result, nil
}

func parsePolicies(path string) (ParseResult, error) {
	result := ParseResult{}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	var policies []AWSPolicy
	if err := json.Unmarshal(data, &policies); err != nil {
		return result, err
	}

	for _, policy := range policies {
		// Create policy node
		result.Nodes = append(result.Nodes, Node{
			ID:     policy.Arn,
			Kind:   KindPolicy,
			Labels: []string{policy.PolicyName, "aws-policy"},
			Props: map[string]string{
				"name": policy.PolicyName,
				"arn":  policy.Arn,
			},
		})

		// Process statements
		for i, stmt := range policy.PolicyVersion.Document.Statement {
			if stmt.Effect != "Allow" {
				continue
			}

			// Parse actions
			actions := parseStringOrArray(stmt.Action)
			resources := parseStringOrArray(stmt.Resource)

			hasWildcard := false
			for _, action := range actions {
				if strings.Contains(action, "*") {
					hasWildcard = true
				}

				// Create permission node
				permID := fmt.Sprintf("%s#stmt%d#%s", policy.Arn, i, action)
				result.Nodes = append(result.Nodes, Node{
					ID:     permID,
					Kind:   KindPerm,
					Labels: []string{action},
					Props: map[string]string{
						"action":   action,
						"wildcard": fmt.Sprintf("%t", strings.Contains(action, "*")),
					},
				})

				// Create ALLOWS_ACTION edge
				result.Edges = append(result.Edges, Edge{
					Src:  policy.Arn,
					Dst:  permID,
					Kind: EdgeAllowsAction,
					Props: map[string]string{
						"statement_index": fmt.Sprintf("%d", i),
					},
				})

				// Create resource nodes and APPLIES_TO edges
				for _, resource := range resources {
					resourceID := resource
					result.Nodes = append(result.Nodes, Node{
						ID:     resourceID,
						Kind:   KindResource,
						Labels: []string{resource},
						Props: map[string]string{
							"arn": resource,
						},
					})

					result.Edges = append(result.Edges, Edge{
						Src:  permID,
						Dst:  resourceID,
						Kind: EdgeAppliesTo,
						Props: map[string]string{
							"action": action,
						},
					})
				}
			}

			if hasWildcard {
				result.Nodes = append(result.Nodes[:len(result.Nodes)-len(resources)], result.Nodes[len(result.Nodes)-len(resources):]...)
			}
		}
	}

	return result, nil
}

func parseAttachments(path string) (ParseResult, error) {
	result := ParseResult{}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	var attachments []AWSAttachment
	if err := json.Unmarshal(data, &attachments); err != nil {
		return result, err
	}

	for _, attachment := range attachments {
		for _, policy := range attachment.AttachedPolicies {
			// Extract account from role name (assuming standard format)
			roleArn := fmt.Sprintf("arn:aws:iam::111111111111:role/%s", attachment.RoleName)
			
			result.Edges = append(result.Edges, Edge{
				Src:  roleArn,
				Dst:  policy.PolicyArn,
				Kind: EdgeAttachedPolicy,
				Props: map[string]string{
					"policy_name": policy.PolicyName,
				},
			})
		}
	}

	return result, nil
}

func parseStringOrArray(raw json.RawMessage) []string {
	var result []string

	// Try string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return []string{str}
	}

	// Try array
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}

	return result
}

