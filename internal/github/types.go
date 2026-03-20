package github

// Issue represents a GitHub issue returned by the REST API.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
}

// PR represents a GitHub pull request.
// HeadSHA is populated from the nested head.sha field in API responses
// and is therefore excluded from direct JSON unmarshalling.
type PR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Draft   bool   `json:"draft"`
	HeadRef string `json:"-"`
	BaseRef string `json:"-"`
	HeadSHA string `json:"-"`
	State   string `json:"state"`
}

// CheckRun represents a single CI check-run associated with a commit.
type CheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// Review represents a pull-request review submitted by a reviewer.
type Review struct {
	State string `json:"state"`
	Body  string `json:"body"`
}

// Tag represents a Git ref created via the git/refs API.
// SHA is populated from the nested object.sha field.
type Tag struct {
	Ref string `json:"ref"`
	SHA string `json:"-"`
}

// Release represents a GitHub release resource.
type Release struct {
	ID      int64  `json:"id"`
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}
