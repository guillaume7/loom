package runtime

import "time"

// ObservationAuthority indicates the reliability level of an observation.
type ObservationAuthority string

const (
	ObservationAuthorityAuthoritative ObservationAuthority = "authoritative"
	ObservationAuthorityDerived       ObservationAuthority = "derived"
)

// ObservationModel aggregates all observations collected during a session.
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

// CheckpointObservation represents a snapshot of the runtime checkpoint.
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

// CIObservation represents CI/check run status.
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

// ReviewObservation represents code review status.
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

// BranchObservation represents git branch metadata.
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

// PRObservation represents pull request state and metadata.
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

// OperatorObservation represents manual operator actions on the workflow.
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

// ObservationSummaries groups latest observations from each source.
type ObservationSummaries struct {
	CI       *CISummary
	Review   *ReviewSummary
	Branch   *BranchSummary
	PR       *PRSummary
	Operator *OperatorSummary
}

// CISummary is the latest recorded CI check status.
type CISummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	Conclusion    string
	Verdict       string
	TotalChecks   int
	GreenChecks   int
}

// ReviewSummary is the latest recorded code review status.
type ReviewSummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	Status        string
	Verdict       string
}

// BranchSummary is the latest recorded branch metadata.
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

// PRSummary is the latest recorded pull request state.
type PRSummary struct {
	Authority     ObservationAuthority
	CorrelationID string
	RecordedAt    time.Time
	PRNumber      int
	Draft         *bool
	WorkflowState string
	Verdict       string
}

// OperatorSummary is the latest recorded operator action.
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
