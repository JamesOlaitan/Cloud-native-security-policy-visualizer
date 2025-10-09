// AccessGraph Sample Neo4j Queries
// These queries demonstrate common access graph analysis patterns

// ========================================
// 1. SHORTEST PATH: PRINCIPAL TO RESOURCE
// ========================================
// Find the shortest path from a specific principal to a resource

MATCH path = shortestPath(
  (principal:Node:K_PRINCIPAL {id: "arn:aws:iam::123456789012:role/DevRole"})-[*]->(resource:Node:K_RESOURCE {id: "arn:aws:s3:::data-bkt"})
)
RETURN path;

// ========================================
// 2. ALL PATHS WITH MAX DEPTH
// ========================================
// Find all paths from a principal to any sensitive resource (max 5 hops)

MATCH path = (principal:Node:K_PRINCIPAL {id: "arn:aws:iam::123456789012:role/DevRole"})-[*1..5]->(resource:Node:K_RESOURCE)
WHERE resource.sensitive = "true"
RETURN path
ORDER BY length(path)
LIMIT 10;

// ========================================
// 3. CROSS-ACCOUNT ACCESS DETECTION
// ========================================
// Find all cross-account assume role relationships

MATCH (principal:Node:K_PRINCIPAL)-[r:K_ASSUMES_ROLE]->(target:Node:K_PRINCIPAL)
WHERE r.cross_account = "true"
RETURN principal.id AS from_principal, 
       target.id AS to_principal,
       r.cross_account AS is_cross_account;

// ========================================
// 4. WILDCARD PERMISSIONS
// ========================================
// Find all policies with wildcard actions

MATCH (principal:Node:K_PRINCIPAL)-[:K_HAS_POLICY]->(policy:Node:K_POLICY)-[access:K_ALLOWS_ACCESS]->(resource:Node:K_RESOURCE)
WHERE access.action = "*" OR access.action ENDS WITH ":*"
RETURN principal.id AS principal,
       policy.id AS policy,
       resource.id AS resource,
       access.action AS wildcard_action;

// ========================================
// 5. PRINCIPALS WITH ACCESS TO SENSITIVE RESOURCES
// ========================================
// List all principals that can reach sensitive resources

MATCH (principal:Node:K_PRINCIPAL)-[*]->(resource:Node:K_RESOURCE)
WHERE resource.sensitive = "true"
RETURN DISTINCT principal.id AS principal,
       principal.name AS principal_name,
       resource.id AS sensitive_resource,
       resource.name AS resource_name;

// ========================================
// 6. UNUSED PERMISSIONS
// ========================================
// Find policies attached to principals that have no outgoing ALLOWS_ACCESS edges
// (This is a simplified heuristic - in reality, usage would need log analysis)

MATCH (principal:Node:K_PRINCIPAL)-[:K_HAS_POLICY]->(policy:Node:K_POLICY)
WHERE NOT (policy)-[:K_ALLOWS_ACCESS]->()
RETURN principal.id AS principal,
       policy.id AS potentially_unused_policy;

// ========================================
// 7. ADMIN PRIVILEGE PATHS
// ========================================
// Find paths involving administrative policies

MATCH path = (principal:Node:K_PRINCIPAL)-[*]->(resource:Node:K_RESOURCE)
WHERE ANY(node IN nodes(path) WHERE node.name IN ["AdministratorAccess", "cluster-admin", "PowerUserAccess"])
RETURN path
LIMIT 20;

// ========================================
// 8. NEIGHBOR ANALYSIS: WHO CAN ACCESS THIS RESOURCE?
// ========================================
// Find all principals with direct or indirect access to a specific resource

MATCH (principal:Node:K_PRINCIPAL)-[*1..3]->(resource:Node:K_RESOURCE {id: "arn:aws:s3:::data-bkt"})
RETURN DISTINCT principal.id AS principal,
       shortestPath((principal)-[*]->(resource)) AS access_path;

// ========================================
// 9. POLICY FANOUT: WHAT DOES THIS POLICY ALLOW?
// ========================================
// Show all resources accessible via a specific policy

MATCH (policy:Node:K_POLICY {id: "arn:aws:iam::123456789012:policy/DevDataAccess"})-[:K_ALLOWS_ACCESS]->(resource:Node:K_RESOURCE)
RETURN policy.id AS policy,
       collect(resource.id) AS accessible_resources,
       count(resource) AS resource_count;

// ========================================
// 10. GRAPH STATISTICS
// ========================================
// Get overall graph statistics

MATCH (n:Node)
WITH n.kind AS kind, count(n) AS node_count
RETURN kind, node_count
ORDER BY node_count DESC

UNION ALL

MATCH ()-[r]->()
WITH type(r) AS relationship_type, count(r) AS rel_count
RETURN relationship_type AS kind, rel_count AS node_count
ORDER BY rel_count DESC;

// ========================================
// 11. BLAST RADIUS: WHAT CAN THIS PRINCIPAL ACCESS?
// ========================================
// Find all resources reachable from a principal (blast radius)

MATCH path = (principal:Node:K_PRINCIPAL {id: "arn:aws:iam::123456789012:role/DevRole"})-[*1..4]->(resource:Node:K_RESOURCE)
RETURN resource.id AS reachable_resource,
       resource.kind AS resource_type,
       length(path) AS hops,
       resource.sensitive AS is_sensitive
ORDER BY hops, reachable_resource;

// ========================================
// 12. LATERAL MOVEMENT OPPORTUNITIES
// ========================================
// Find principals that can assume other principals (lateral movement)

MATCH path = (source:Node:K_PRINCIPAL)-[:K_ASSUMES_ROLE*]->(target:Node:K_PRINCIPAL)
WHERE source <> target
RETURN DISTINCT source.id AS source_principal,
       target.id AS target_principal,
       length(path) AS hops
ORDER BY hops
LIMIT 20;

// ========================================
// 13. HIGH-RISK PATHS
// ========================================
// Find paths with both cross-account access AND wildcard permissions

MATCH path = (principal:Node:K_PRINCIPAL)-[*]->(resource:Node:K_RESOURCE)
WHERE ANY(rel IN relationships(path) WHERE rel.cross_account = "true")
  AND ANY(rel IN relationships(path) WHERE rel.action = "*" OR rel.action ENDS WITH ":*")
RETURN path
LIMIT 10;

// ========================================
// 14. KUBERNETES CLUSTER-ADMIN BINDINGS
// ========================================
// Find all service accounts with cluster-admin role

MATCH (sa:Node:K_PRINCIPAL)-[:K_HAS_ROLE]->(role:Node:K_POLICY)
WHERE role.name = "cluster-admin"
RETURN sa.id AS service_account,
       sa.namespace AS namespace,
       role.id AS cluster_admin_role;

// ========================================
// 15. PAGERANK: MOST IMPORTANT RESOURCES
// ========================================
// Use PageRank to find the most "important" nodes in the graph
// (Most connected/central resources)

CALL gds.pageRank.stream('accessgraph')
YIELD nodeId, score
WITH gds.util.asNode(nodeId) AS node, score
WHERE node.kind = "RESOURCE"
RETURN node.id AS resource,
       node.name AS name,
       score
ORDER BY score DESC
LIMIT 10;

// Note: The last query requires Neo4j Graph Data Science (GDS) library
// Install with: CALL gds.graph.project('accessgraph', 'Node', {K_HAS_POLICY: {}, K_ALLOWS_ACCESS: {}, K_ASSUMES_ROLE: {}})

