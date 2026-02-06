package accessgraph

test_cross_account_detection {
    count(violations) > 0 with input as {
        "policies": {},
        "roles": {
            "role1": {
                "name": "CrossAccountRole",
                "trust": {"cross_account": true}
            }
        },
        "k8s": {"bindings": {}}
    }
}

test_no_cross_account_no_violation {
    count(violations) == 0 with input as {
        "policies": {},
        "roles": {
            "role1": {
                "name": "SameAccountRole",
                "trust": {"cross_account": false}
            }
        },
        "k8s": {"bindings": {}}
    }
}
