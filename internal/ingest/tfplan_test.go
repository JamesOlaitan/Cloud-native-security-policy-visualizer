package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTerraform(t *testing.T) {
	tmpDir := t.TempDir()

	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.5.0",
		"planned_values": {
			"root_module": {
				"resources": [{
					"address": "aws_iam_policy.test",
					"type": "aws_iam_policy",
					"name": "test",
					"values": {
						"name": "TestPolicy",
						"policy": "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Action\":\"s3:*\",\"Resource\":[\"arn:aws:s3:::test-bkt\"]}]}"
					}
				}]
			}
		},
		"resource_changes": [{
			"address": "aws_iam_policy.test",
			"type": "aws_iam_policy",
			"change": {
				"actions": ["update"],
				"before": {
					"policy": "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Action\":\"s3:GetObject\",\"Resource\":[\"arn:aws:s3:::test-bkt/*\"]}]}"
				},
				"after": {
					"policy": "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Action\":\"s3:*\",\"Resource\":[\"arn:aws:s3:::test-bkt\",\"arn:aws:s3:::test-bkt/*\"]}]}"
				}
			}
		}]
	}`

	planPath := filepath.Join(tmpDir, "plan.json")
	os.WriteFile(planPath, []byte(planJSON), 0644)

	result, isTF, err := ParseTerraform(planPath)
	if err != nil {
		t.Fatalf("ParseTerraform failed: %v", err)
	}

	if !isTF {
		t.Error("Expected isTF to be true")
	}

	if len(result.Nodes) == 0 {
		t.Error("Expected nodes from Terraform plan")
	}

	// Check for wildcard detection
	hasWildcard := false
	for _, node := range result.Nodes {
		if node.Kind == KindPerm && node.Props["wildcard"] == "true" {
			hasWildcard = true
			break
		}
	}

	if !hasWildcard {
		t.Error("Expected wildcard permission node")
	}
}

func TestParseTerraformMissing(t *testing.T) {
	result, isTF, err := ParseTerraform("/nonexistent/plan.json")
	if err != nil {
		t.Errorf("Expected no error for missing file, got: %v", err)
	}

	if isTF {
		t.Error("Expected isTF to be false for missing file")
	}

	if len(result.Nodes) != 0 {
		t.Error("Expected no nodes for missing file")
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !contains(slice, "b") {
		t.Error("Expected contains to find 'b'")
	}

	if contains(slice, "d") {
		t.Error("Expected contains not to find 'd'")
	}
}
