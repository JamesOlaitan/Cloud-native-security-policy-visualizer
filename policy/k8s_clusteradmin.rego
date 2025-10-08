package accessgraph

# Kubernetes ClusterAdmin Binding Detection
violations[result] {
    binding := input.k8s.bindings[binding_id]
    binding.cluster_admin == true
    
    result := {
        "ruleId": "K8s.ClusterAdminBinding",
        "severity": "HIGH",
        "entityRef": binding_id,
        "reason": sprintf("Binding '%s' grants cluster-admin role, providing unrestricted cluster access", [binding.name]),
        "remediation": "Replace cluster-admin with namespace-scoped roles or specific ClusterRoles with minimal required permissions. Avoid wildcard permissions"
    }
}

