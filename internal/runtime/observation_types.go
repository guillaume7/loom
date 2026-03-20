package runtime

import "time"

type ObservationAuthority string

const (
	ObservationAuthorityAuthoritative ObservationAuthority = "authoritative"
	ObservationAuthorityDerived       ObservationAuthority = "derived"
)

type ObservationModel struct {
	SessionID  string
	Checkpoint CheckpointObservation
	CI         []CIObservation
	Review     []ReviewObservation
	Branch     []BranchObservation
	PR         []PRObservation
	Operator   []OperatorObservation
	Summaries  ObservationSummaries
}

type CheckpointObservation struct {
	Authority   ObservationAuthority
	State       string
	ResumeState string
	Phase       int
	PRNumber    int
	IssueNumber int
	RetryCount  int
	UpdatedAt   time.Time
}

type CIObservation struct {
	Authority     ObservationAuthority
	Source        string
	CorrelationID string
	ObservedAt    time.Time
	EventKind     string
	Conclusion    string
	Verdict       string
	TotalChecks   int
	GreenChecks   int
	PendingChecks []string
	FailedChecks  []string
}

type ReviewObservation struct {
	Authority     ObservationAuthority
	Source        string
	CorrelationID string
	ObservedAt    time.Time
	EventKind     string
	Status        string
	Verdict       string
	Action        string
}

type BranchObservation struct {
	Authority     ObservationAuthority
	Source        string
	CorrelationID string
	ObservedAt    time.Time
	Repository    string
	Ref           string
	BaseRef       string
	HeadRef       string
	HeadSHA       string
}

type PRObservation struct {
	Authority     ObservationAuthority
	Source        string
	CorrelationID string
	ObservedAt    time.Time
	EventKind     string
	Action        string
	Adapter       string
	WorkflowState string
	PRNumber      int
	Draft         *bool
	Verdict       string
}

type OperatorObservation struct {
	Authority     ObservationAuthority
	Source        string
	CorrelationID string
	ObservedAt    time.Time
	Action        string
	RequestedBy   string
	Reason        string
	PreviousState string
	NewState      string
	ResumeState   string
}

type ObservationSummaries struct {
	CI       *CISummary
	Review   *ReviewSummary
	Branch   *BranchSummary
	PR       *PRSummary
	Operator *OperatorSummary
}

type CISummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	Conclusion    string
	Verdict       string
	TotalChecks   int
	GreenChecks   int
}

type ReviewSummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	Status        string
	Verdict       string
}

type BranchSummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	Repository    string
	Ref           string
	BaseRef       string
	HeadRef       string
	HeadSHA       string
}

type PRSummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	PRNumber      int
	Draft         *bool
	WorkflowState string
	Verdict       string
}

type OperatorSummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	Action        string
	RequestedBy   string
	Reason        string
}

type githubPayloadSnapshot struct {
	Ref         string `json:"ref"`
	State       string `json:"state"`
	PullRequest struct {
		Draft *bool `json:"draft,omitempty"`
		Head  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
	} `json:"pull_request"`
	Review struct {
		State string `json:"state"`
	} `json:"review"`
	CheckSuite struct {
		Conclusion string `json:"conclusion"`
		HeadBranch string `json:"head_branch"`
		HeadSHA    string `json:"head_sha"`
	} `json:"check_suite"`
	CheckRun struct {
		Conclusion string `json:"conclusion"`
		CheckSuite struct {
			HeadBranch string `json:"head_branch"`
			HeadSHA    string `json:"head_sha"`
		} `json:"check_suite"`
	} `json:"check_run"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"repository"`
}
