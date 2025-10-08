package accessgraph

# Cross-Account Trust Detection
violations[result] {
    role := input.roles[role_id]
    role.trust.cross_account == true
    
    result := {
        "ruleId": "IAM.CrossAccountAssumeRole",
        "severity": "HIGH",
        "entityRef": role_id,
        "reason": sprintf("Role '%s' trusts a principal from another AWS account, creating cross-account access", [role.name]),
        "remediation": "Review cross-account trust relationships. Ensure external principals are authorized and consider adding conditions like ExternalId or MFA requirements"
    }
}

