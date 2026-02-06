package accessgraph

test_clusteradmin_detection {
    count(violations) > 0 with input as {
        "policies": {},
        "roles": {},
        "k8s": {
            "bindings": {
                "binding1": {
                    "name": "admin-binding",
                    "cluster_admin": true
                }
            }
        }
    }
}

test_no_clusteradmin_no_violation {
    count(violations) == 0 with input as {
        "policies": {},
        "roles": {},
        "k8s": {
            "bindings": {
                "binding1": {
                    "name": "readonly-binding",
                    "cluster_admin": false
                }
            }
        }
    }
}
