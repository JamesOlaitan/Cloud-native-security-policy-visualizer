package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// K8sResource represents a generic Kubernetes resource
type K8sResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Subjects []struct {
		Kind      string `yaml:"kind"`
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"subjects"`
	RoleRef struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`
	} `yaml:"roleRef"`
	Rules []struct {
		APIGroups []string `yaml:"apiGroups"`
		Resources []string `yaml:"resources"`
		Verbs     []string `yaml:"verbs"`
	} `yaml:"rules"`
	Spec struct {
		PodSelector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"podSelector"`
	} `yaml:"spec"`
}

// ParseK8s parses Kubernetes RBAC YAML files from a directory
func ParseK8s(dirPath string) (ParseResult, error) {
	result := ParseResult{
		Nodes: []Node{},
		Edges: []Edge{},
	}

	files := []string{
		"serviceaccounts.yaml",
		"clusterroles.yaml",
		"rolebindings.yaml",
		"networkpolicies.yaml",
	}

	for _, file := range files {
		path := filepath.Join(dirPath, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", file, err)
		}

		// Parse YAML documents
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		for {
			var resource K8sResource
			if err := decoder.Decode(&resource); err != nil {
				break
			}

			parsed := parseK8sResource(resource)
			result.Merge(parsed)
		}
	}

	return result, nil
}

func parseK8sResource(resource K8sResource) ParseResult {
	result := ParseResult{}

	switch resource.Kind {
	case "ServiceAccount":
		result.Nodes = append(result.Nodes, Node{
			ID:     fmt.Sprintf("k8s:sa:%s:%s", resource.Metadata.Namespace, resource.Metadata.Name),
			Kind:   KindPrincipal,
			Labels: []string{resource.Metadata.Name, "k8s-serviceaccount"},
			Props: map[string]string{
				"name":      resource.Metadata.Name,
				"namespace": resource.Metadata.Namespace,
			},
		})

		// Create namespace node
		if resource.Metadata.Namespace != "" {
			result.Nodes = append(result.Nodes, Node{
				ID:     fmt.Sprintf("k8s:ns:%s", resource.Metadata.Namespace),
				Kind:   KindNS,
				Labels: []string{resource.Metadata.Namespace},
				Props: map[string]string{
					"name": resource.Metadata.Namespace,
				},
			})

			result.Edges = append(result.Edges, Edge{
				Src:   fmt.Sprintf("k8s:sa:%s:%s", resource.Metadata.Namespace, resource.Metadata.Name),
				Dst:   fmt.Sprintf("k8s:ns:%s", resource.Metadata.Namespace),
				Kind:  EdgeInNamespace,
				Props: map[string]string{},
			})
		}

	case "ClusterRole", "Role":
		roleID := fmt.Sprintf("k8s:role:%s", resource.Metadata.Name)
		if resource.Kind == "Role" && resource.Metadata.Namespace != "" {
			roleID = fmt.Sprintf("k8s:role:%s:%s", resource.Metadata.Namespace, resource.Metadata.Name)
		}

		isClusterAdmin := resource.Metadata.Name == "cluster-admin"

		result.Nodes = append(result.Nodes, Node{
			ID:     roleID,
			Kind:   KindRole,
			Labels: []string{resource.Metadata.Name, fmt.Sprintf("k8s-%s", strings.ToLower(resource.Kind))},
			Props: map[string]string{
				"name":          resource.Metadata.Name,
				"cluster_admin": fmt.Sprintf("%t", isClusterAdmin),
			},
		})

		// Process rules
		for i, rule := range resource.Rules {
			for _, verb := range rule.Verbs {
				for _, res := range rule.Resources {
					permID := fmt.Sprintf("%s#rule%d#%s#%s", roleID, i, verb, res)

					isWildcard := verb == "*" || res == "*"

					result.Nodes = append(result.Nodes, Node{
						ID:     permID,
						Kind:   KindPerm,
						Labels: []string{fmt.Sprintf("%s:%s", verb, res)},
						Props: map[string]string{
							"verb":     verb,
							"resource": res,
							"wildcard": fmt.Sprintf("%t", isWildcard),
						},
					})

					result.Edges = append(result.Edges, Edge{
						Src:  roleID,
						Dst:  permID,
						Kind: EdgeAllowsAction,
						Props: map[string]string{
							"rule_index": fmt.Sprintf("%d", i),
						},
					})
				}
			}
		}

	case "ClusterRoleBinding", "RoleBinding":
		bindingID := fmt.Sprintf("k8s:binding:%s", resource.Metadata.Name)
		roleID := fmt.Sprintf("k8s:role:%s", resource.RoleRef.Name)

		for _, subject := range resource.Subjects {
			var subjectID string
			if subject.Kind == "ServiceAccount" {
				ns := subject.Namespace
				if ns == "" {
					ns = resource.Metadata.Namespace
				}
				subjectID = fmt.Sprintf("k8s:sa:%s:%s", ns, subject.Name)
			} else {
				subjectID = fmt.Sprintf("k8s:%s:%s", strings.ToLower(subject.Kind), subject.Name)
			}

			result.Edges = append(result.Edges, Edge{
				Src:  roleID,
				Dst:  subjectID,
				Kind: EdgeBindsTo,
				Props: map[string]string{
					"binding": bindingID,
				},
			})
		}

	case "NetworkPolicy":
		// Store metadata only
		npID := fmt.Sprintf("k8s:netpol:%s:%s", resource.Metadata.Namespace, resource.Metadata.Name)
		labels := []string{resource.Metadata.Name}
		for k, v := range resource.Metadata.Labels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}

		result.Nodes = append(result.Nodes, Node{
			ID:     npID,
			Kind:   KindResource,
			Labels: labels,
			Props: map[string]string{
				"name":      resource.Metadata.Name,
				"namespace": resource.Metadata.Namespace,
				"type":      "NetworkPolicy",
			},
		})
	}

	return result
}
