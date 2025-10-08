package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseK8s(t *testing.T) {
	tmpDir := t.TempDir()

	saYAML := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
  namespace: default
`

	roleYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-admin
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
`

	bindingYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: test-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: default
`

	os.WriteFile(filepath.Join(tmpDir, "serviceaccounts.yaml"), []byte(saYAML), 0644)
	os.WriteFile(filepath.Join(tmpDir, "clusterroles.yaml"), []byte(roleYAML), 0644)
	os.WriteFile(filepath.Join(tmpDir, "rolebindings.yaml"), []byte(bindingYAML), 0644)

	result, err := ParseK8s(tmpDir)
	if err != nil {
		t.Fatalf("ParseK8s failed: %v", err)
	}

	if len(result.Nodes) == 0 {
		t.Error("Expected nodes, got 0")
	}

	// Check for cluster-admin role
	hasClusterAdmin := false
	for _, node := range result.Nodes {
		if node.Kind == KindRole && node.Props["cluster_admin"] == "true" {
			hasClusterAdmin = true
			break
		}
	}

	if !hasClusterAdmin {
		t.Error("Expected cluster-admin role")
	}

	// Check for binding edge
	hasBinding := false
	for _, edge := range result.Edges {
		if edge.Kind == EdgeBindsTo {
			hasBinding = true
			break
		}
	}

	if !hasBinding {
		t.Error("Expected binding edge")
	}
}

