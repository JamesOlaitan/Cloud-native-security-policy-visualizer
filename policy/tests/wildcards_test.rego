package accessgraph

test_wildcard_detection {
    count(violations) > 0 with input as {
        "policies": {
            "policy1": {
                "name": "TestPolicy",
                "action_matches_wildcard": true
            }
        },
        "roles": {},
        "k8s": {"bindings": {}}
    }
}

test_no_wildcard_no_violation {
    count(violations) == 0 with input as {
        "policies": {
            "policy1": {
                "name": "TestPolicy",
                "action_matches_wildcard": false
            }
        },
        "roles": {},
        "k8s": {"bindings": {}}
    }
}
