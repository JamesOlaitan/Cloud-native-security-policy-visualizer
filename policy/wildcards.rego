package accessgraph

# IAM Wildcard Action Detection
violations[result] {
    policy := input.policies[policy_id]
    policy.action_matches_wildcard == true
    
    result := {
        "ruleId": "IAM.WildcardAction",
        "severity": "MEDIUM",
        "entityRef": policy_id,
        "reason": sprintf("Policy '%s' contains wildcard (*) in actions, granting overly broad permissions", [policy.name]),
        "remediation": "Replace wildcard actions with specific, least-privilege permissions. List only the required actions (e.g., s3:GetObject, s3:PutObject) instead of s3:*"
    }
}

