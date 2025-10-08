package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAWS(t *testing.T) {
	// Create temp directory with sample files
	tmpDir := t.TempDir()

	rolesJSON := `[{
		"RoleName": "TestRole",
		"Arn": "arn:aws:iam::111111111111:role/TestRole",
		"AssumeRolePolicyDocument": {
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": "arn:aws:iam::222222222222:role/ExtRole"},
				"Action": "sts:AssumeRole"
			}]
		}
	}]`

	policiesJSON := `[{
		"PolicyName": "TestPolicy",
		"Arn": "arn:aws:iam::111111111111:policy/TestPolicy",
		"PolicyVersion": {
			"Document": {
				"Statement": [{
					"Effect": "Allow",
					"Action": "s3:*",
					"Resource": "arn:aws:s3:::test-bucket"
				}]
			}
		}
	}]`

	attachmentsJSON := `[{
		"RoleName": "TestRole",
		"AttachedPolicies": [{
			"PolicyName": "TestPolicy",
			"PolicyArn": "arn:aws:iam::111111111111:policy/TestPolicy"
		}]
	}]`

	os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte(rolesJSON), 0644)
	os.WriteFile(filepath.Join(tmpDir, "policies.json"), []byte(policiesJSON), 0644)
	os.WriteFile(filepath.Join(tmpDir, "attachments.json"), []byte(attachmentsJSON), 0644)

	result, err := ParseAWS(tmpDir)
	if err != nil {
		t.Fatalf("ParseAWS failed: %v", err)
	}

	if len(result.Nodes) == 0 {
		t.Error("Expected nodes, got 0")
	}

	if len(result.Edges) == 0 {
		t.Error("Expected edges, got 0")
	}

	// Check for cross-account trust
	hasCrossAccountEdge := false
	for _, edge := range result.Edges {
		if edge.Kind == EdgeTrustsCrossAccount {
			hasCrossAccountEdge = true
			break
		}
	}

	if !hasCrossAccountEdge {
		t.Error("Expected cross-account trust edge")
	}
}

func TestParseStringOrArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "string",
			input:    `"s3:GetObject"`,
			expected: []string{"s3:GetObject"},
		},
		{
			name:     "array",
			input:    `["s3:GetObject", "s3:PutObject"]`,
			expected: []string{"s3:GetObject", "s3:PutObject"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStringOrArray([]byte(tt.input))
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
			}
		})
	}
}

