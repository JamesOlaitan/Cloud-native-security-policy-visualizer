package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is an OPA HTTP client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Finding represents a policy violation
type Finding struct {
	RuleID      string `json:"ruleId"`
	Severity    string `json:"severity"`
	EntityRef   string `json:"entityRef"`
	Reason      string `json:"reason"`
	Remediation string `json:"remediation"`
}

// OPAResponse represents the response from OPA
type OPAResponse struct {
	Result struct {
		Violations []Finding `json:"violations"`
	} `json:"result"`
}

// NewClient creates a new OPA client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Evaluate sends input to OPA and returns findings
func (c *Client) Evaluate(input map[string]interface{}) ([]Finding, error) {
	inputWrapper := map[string]interface{}{
		"input": input,
	}

	body, err := json.Marshal(inputWrapper)
	if err != nil {
		return nil, fmt.Errorf("marshaling input: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("calling OPA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OPA returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var opaResp OPAResponse
	if err := json.NewDecoder(resp.Body).Decode(&opaResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return opaResp.Result.Violations, nil
}
