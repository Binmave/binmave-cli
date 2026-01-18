package api

import "time"

// Agent represents an agent in the system
type Agent struct {
	AgentID                   string    `json:"agentId"`
	MachineName               string    `json:"machineName"`
	OperatingSystem           string    `json:"operatingSystem"`
	AgentVersion              string    `json:"agentVersion"`
	LastIP                    string    `json:"lastIP"`
	LastTransportType         string    `json:"lastTransportType"`
	AgentStatus               string    `json:"agentStatus"`
	ConnectionID              string    `json:"connectionId"`
	LastConnectionEstablished time.Time `json:"lastConnectionEstablished"`
	AgentConfigName           string    `json:"agentConfigName"`
	CapabilityNames           []string  `json:"capabilityNames"`
}

// AgentStats represents agent statistics
type AgentStats struct {
	Total             int            `json:"total"`
	Online            int            `json:"online"`
	Offline           int            `json:"offline"`
	Expired           int            `json:"expired"`
	Expiring          int            `json:"expiring"`
	ByOperatingSystem map[string]int `json:"byOperatingSystem"`
}

// Script represents a script in the system
type Script struct {
	ScriptID      int      `json:"scriptId"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       int      `json:"version"`
	Tags          []string `json:"tags"`
	ScriptType    string   `json:"scriptType"`
	OutputType    string   `json:"outputType"`
	ScriptTimeout string   `json:"scriptTimeout"`
	RepoName      string   `json:"repoName"`
	RepoType      string   `json:"repoType"`
}

// ScriptInput represents an input parameter for a script
type ScriptInput struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Type     string `json:"type"`
	Required bool   `json:"isRequired"`
}

// ExecuteRequest represents a request to execute a script
type ExecuteRequest struct {
	ScriptTimeout    string        `json:"scriptTimeout,omitempty"`
	FilterGridString string        `json:"filterGridString"`
	Inputs           []ScriptInput `json:"inputs,omitempty"`
	CleanSandBox     bool          `json:"cleanSandBox"`
}

// ExecuteResponse represents the response from executing a script
type ExecuteResponse struct {
	ExecutionID    string    `json:"executionId"`
	ScriptID       int       `json:"scriptId"`
	ScriptName     string    `json:"scriptName"`
	Created        time.Time `json:"created"`
	ExpectedAgents int       `json:"expectedAgents"`
}

// Execution represents a script execution
type Execution struct {
	ExecutionID      string        `json:"executionId"`
	ScriptID         int           `json:"scriptId"`
	ScriptName       string        `json:"scriptName"`
	Created          time.Time     `json:"created"`
	CreatedBy        string        `json:"createdBy"`
	ScriptTimeout    string        `json:"scriptTimeout"`
	FilterGridString string        `json:"filterGridString"`
	Inputs           []ScriptInput `json:"inputs"`
}

// ExecutionStatus represents the status of an execution
type ExecutionStatus struct {
	Expected int    `json:"expected"`
	Received int    `json:"received"`
	Errors   int    `json:"errors"`
	State    string `json:"state"` // Pending, Running, Completed, Failed
}

// ExecutionListItem represents an execution in a list view
type ExecutionListItem struct {
	ExecutionID string    `json:"executionId"`
	ScriptID    int       `json:"scriptId"`
	ScriptName  string    `json:"scriptName"`
	Created     time.Time `json:"created"`
	CreatedBy   string    `json:"createdBy"`
	Expected    int       `json:"expected"`
	Received    int       `json:"received"`
	Errors      int       `json:"errors"`
	State       string    `json:"state"`
}

// ExecutionResult represents a single result from an agent
type ExecutionResult struct {
	ResultID             int       `json:"resultId"`
	AgentID              string    `json:"agentId"`
	AgentName            string    `json:"agentName"`
	AnswerJSON           string    `json:"answerJson"`
	RawStdError          string    `json:"rawStdError"`
	ExecutionTimeSeconds int       `json:"executionTimeSeconds"`
	ResultGenerated      time.Time `json:"resultGenerated"`
	ResultReceived       time.Time `json:"resultReceived"`
	HasError             bool      `json:"hasError"`
}

// ExecutionResultsPage represents a paginated list of results
type ExecutionResultsPage struct {
	Results    []ExecutionResult `json:"results"`
	TotalCount int               `json:"totalCount"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
}

// LoadResult represents a DevExtreme load result
type LoadResult struct {
	Data       interface{} `json:"data"`
	TotalCount int         `json:"totalCount"`
}
