package github

import (
	"context"
	"net/http"
	"time"

	gh "github.com/google/go-github/v90/github"
)

// Repository is a repository the user can reach.
type Repository struct {
	Repo          Repo
	Description   string
	Private       bool
	DefaultBranch string
	CloneURL      string
	SSHURL        string
	HTMLURL       string
	UpdatedAt     time.Time
}

func repository(r *gh.Repository) Repository {
	return Repository{
		Repo:        Repo{Owner: r.GetOwner().GetLogin(), Name: r.GetName()},
		Description: r.GetDescription(), Private: r.GetPrivate(), DefaultBranch: r.GetDefaultBranch(),
		CloneURL: r.GetCloneURL(), SSHURL: r.GetSSHURL(), HTMLURL: r.GetHTMLURL(), UpdatedAt: r.GetUpdatedAt().Time,
	}
}

// NewRepository describes a repository to create.
type NewRepository struct {
	// Org creates it in an organization; empty is the user's own account.
	Org         string
	Name        string
	Description string
	Private     bool
	// README, Gitignore ("Go", "Node", …) and License ("mit", …) make
	// GitHub create a first commit. Leave them empty when an existing local
	// repository will be pushed.
	README    bool
	Gitignore string
	License   string
}

// CreateRepository creates a repository on GitHub.
func (c *Client) CreateRepository(ctx context.Context, n NewRepository) (Repository, error) {
	r := &gh.Repository{Name: gh.Ptr(n.Name), Private: gh.Ptr(n.Private)}
	if n.Description != "" {
		r.Description = gh.Ptr(n.Description)
	}
	if n.README {
		r.AutoInit = gh.Ptr(true)
	}
	if n.Gitignore != "" {
		r.GitignoreTemplate = gh.Ptr(n.Gitignore)
	}
	if n.License != "" {
		r.LicenseTemplate = gh.Ptr(n.License)
	}
	created, _, err := c.gh.Repositories.Create(ctx, n.Org, r)
	if err != nil {
		return Repository{}, classify(ctx, err)
	}
	return repository(created), nil
}

// Repositories lists the user's repositories, most recently updated first.
func (c *Client) Repositories(ctx context.Context, limit int) ([]Repository, error) {
	var out []Repository
	opts := &gh.RepositoryListByAuthenticatedUserOptions{Sort: "updated", ListOptions: gh.ListOptions{PerPage: 100}}
	for {
		page, resp, err := c.gh.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, classify(ctx, err)
		}
		for _, r := range page {
			out = append(out, repository(r))
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		if resp.NextPage == 0 {
			return out, nil
		}
		opts.Page = resp.NextPage
	}
}

// PullRequest is a pull request.
type PullRequest struct {
	Number    int
	Title     string
	Body      string
	State     string
	Draft     bool
	Merged    bool
	Head      string
	Base      string
	Author    string
	HTMLURL   string
	Mergeable *bool
	CreatedAt time.Time
}

func pullRequest(p *gh.PullRequest) PullRequest {
	return PullRequest{
		Number: p.GetNumber(), Title: p.GetTitle(), Body: p.GetBody(), State: p.GetState(), Draft: p.GetDraft(),
		Merged: p.GetMerged(), Head: p.GetHead().GetRef(), Base: p.GetBase().GetRef(), Author: p.GetUser().GetLogin(),
		HTMLURL: p.GetHTMLURL(), Mergeable: p.Mergeable, CreatedAt: p.GetCreatedAt().Time,
	}
}

// NewPullRequest describes a pull request to open.
type NewPullRequest struct {
	Title, Body, Head, Base string
	Draft                   bool
}

// CreatePullRequest opens a pull request.
func (c *Client) CreatePullRequest(ctx context.Context, repo Repo, n NewPullRequest) (PullRequest, error) {
	p, _, err := c.gh.PullRequests.Create(ctx, repo.Owner, repo.Name, gh.CreatePullRequest{
		Title: gh.Ptr(n.Title), Body: gh.Ptr(n.Body), Head: n.Head, Base: n.Base, Draft: gh.Ptr(n.Draft),
	})
	if err != nil {
		return PullRequest{}, classify(ctx, err)
	}
	return pullRequest(p), nil
}

// PullRequests lists pull requests in a state: "open", "closed" or "all".
func (c *Client) PullRequests(ctx context.Context, repo Repo, state string, limit int) ([]PullRequest, error) {
	if state == "" {
		state = "open"
	}
	page, _, err := c.gh.PullRequests.List(ctx, repo.Owner, repo.Name, &gh.PullRequestListOptions{
		State: state, ListOptions: gh.ListOptions{PerPage: max(1, min(limit, 100))},
	})
	if err != nil {
		return nil, classify(ctx, err)
	}
	out := make([]PullRequest, 0, len(page))
	for _, p := range page {
		out = append(out, pullRequest(p))
	}
	return out, nil
}

// PullRequestByNumber returns one pull request.
func (c *Client) PullRequestByNumber(ctx context.Context, repo Repo, number int) (PullRequest, error) {
	p, _, err := c.gh.PullRequests.Get(ctx, repo.Owner, repo.Name, number)
	if err != nil {
		return PullRequest{}, classify(ctx, err)
	}
	return pullRequest(p), nil
}

// MergeMethod is how GitHub merges a pull request.
type MergeMethod string

// Merge methods.
const (
	MergeCommit MergeMethod = "merge"
	MergeSquash MergeMethod = "squash"
	MergeRebase MergeMethod = "rebase"
)

// MergePullRequest merges a pull request on GitHub and returns the merge
// commit.
func (c *Client) MergePullRequest(ctx context.Context, repo Repo, number int, method MergeMethod) (string, error) {
	if method == "" {
		method = MergeCommit
	}
	res, _, err := c.gh.PullRequests.Merge(ctx, repo.Owner, repo.Name, number, "", &gh.PullRequestOptions{MergeMethod: string(method)})
	if err != nil {
		return "", classify(ctx, err)
	}
	if !res.GetMerged() {
		return "", &Error{Kind: KindNotMerged, Detail: res.GetMessage()}
	}
	return res.GetSHA(), nil
}

// ClosePullRequest closes a pull request without merging it.
func (c *Client) ClosePullRequest(ctx context.Context, repo Repo, number int) error {
	_, _, err := c.gh.PullRequests.Edit(ctx, repo.Owner, repo.Name, number, &gh.PullRequest{State: gh.Ptr("closed")})
	return classify(ctx, err)
}

// Comment adds a comment to a pull request or issue.
func (c *Client) Comment(ctx context.Context, repo Repo, number int, body string) error {
	_, _, err := c.gh.Issues.CreateComment(ctx, repo.Owner, repo.Name, number, &gh.IssueComment{Body: gh.Ptr(body)})
	return classify(ctx, err)
}

// Issue is an issue.
type Issue struct {
	Number  int
	Title   string
	Body    string
	State   string
	Labels  []string
	Author  string
	HTMLURL string
}

func issue(i *gh.Issue) Issue {
	out := Issue{
		Number: i.GetNumber(), Title: i.GetTitle(), Body: i.GetBody(), State: i.GetState(),
		Author: i.GetUser().GetLogin(), HTMLURL: i.GetHTMLURL(),
	}
	for _, l := range i.Labels {
		out.Labels = append(out.Labels, l.GetName())
	}
	return out
}

// Issues lists issues in a state ("open", "closed", "all"). Pull requests,
// which GitHub also lists as issues, are left out.
func (c *Client) Issues(ctx context.Context, repo Repo, state string, limit int) ([]Issue, error) {
	if state == "" {
		state = "open"
	}
	page, _, err := c.gh.Issues.ListByRepo(ctx, repo.Owner, repo.Name, &gh.IssueListByRepoOptions{
		State: state, ListOptions: gh.ListOptions{PerPage: max(1, min(limit, 100))},
	})
	if err != nil {
		return nil, classify(ctx, err)
	}
	var out []Issue
	for _, i := range page {
		if i.IsPullRequest() {
			continue
		}
		out = append(out, issue(i))
	}
	return out, nil
}

// CreateIssue opens an issue.
func (c *Client) CreateIssue(ctx context.Context, repo Repo, title, body string, labels []string) (Issue, error) {
	i, _, err := c.gh.Issues.Create(ctx, repo.Owner, repo.Name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: labels})
	if err != nil {
		return Issue{}, classify(ctx, err)
	}
	return issue(i), nil
}

// CloseIssue closes an issue.
func (c *Client) CloseIssue(ctx context.Context, repo Repo, number int) error {
	_, _, err := c.gh.Issues.Update(ctx, repo.Owner, repo.Name, number, gh.UpdateIssueRequest{State: gh.Ptr("closed")})
	return classify(ctx, err)
}

// LabelIssue adds labels to an issue or pull request.
func (c *Client) LabelIssue(ctx context.Context, repo Repo, number int, labels []string) error {
	_, _, err := c.gh.Issues.AddLabelsToIssue(ctx, repo.Owner, repo.Name, number, labels)
	return classify(ctx, err)
}

// NewRelease describes a release made from a tag.
type NewRelease struct {
	Tag, Name, Body string
	Draft           bool
	Prerelease      bool
	// GenerateNotes lets GitHub write the notes from merged pull requests.
	GenerateNotes bool
}

// CreateRelease publishes a release; it returns its page.
func (c *Client) CreateRelease(ctx context.Context, repo Repo, n NewRelease) (string, error) {
	req := gh.CreateReleaseRequest{
		TagName: n.Tag, Draft: gh.Ptr(n.Draft), Prerelease: gh.Ptr(n.Prerelease), GenerateReleaseNotes: gh.Ptr(n.GenerateNotes),
	}
	if n.Name != "" {
		req.Name = gh.Ptr(n.Name)
	}
	if n.Body != "" {
		req.Body = gh.Ptr(n.Body)
	}
	r, _, err := c.gh.Repositories.CreateRelease(ctx, repo.Owner, repo.Name, req)
	if err != nil {
		return "", classify(ctx, err)
	}
	return r.GetHTMLURL(), nil
}

// Run is one GitHub Actions workflow run.
type Run struct {
	Workflow   string
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, cancelled, …
	Branch     string
	Commit     string
	HTMLURL    string
	CreatedAt  time.Time
}

// WorkflowRuns lists the latest workflow runs, of a branch when given.
func (c *Client) WorkflowRuns(ctx context.Context, repo Repo, branch string, limit int) ([]Run, error) {
	runs, _, err := c.gh.Actions.ListRepositoryWorkflowRuns(ctx, repo.Owner, repo.Name, &gh.ListWorkflowRunsOptions{
		Branch: branch, ListOptions: gh.ListOptions{PerPage: max(1, min(limit, 100))},
	})
	if err != nil {
		return nil, classify(ctx, err)
	}
	out := make([]Run, 0, len(runs.WorkflowRuns))
	for _, r := range runs.WorkflowRuns {
		out = append(out, Run{
			Workflow: r.GetName(), Status: r.GetStatus(), Conclusion: r.GetConclusion(), Branch: r.GetHeadBranch(),
			Commit: r.GetHeadSHA(), HTMLURL: r.GetHTMLURL(), CreatedAt: r.GetCreatedAt().Time,
		})
	}
	return out, nil
}

// BranchProtected reports whether GitHub protects a branch, so a push to it
// can be warned about before it is rejected (spec §5).
func (c *Client) BranchProtected(ctx context.Context, repo Repo, branch string) (bool, error) {
	b, resp, err := c.gh.Repositories.GetBranch(ctx, repo.Owner, repo.Name, branch, 1)
	if err != nil {
		// GetBranch reports a status it did not expect as a plain error.
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil // a branch not pushed yet
		}
		if resp != nil {
			return false, classifyStatus(resp.StatusCode)
		}
		return false, classify(ctx, err)
	}
	return b.GetProtected(), nil
}

// Repository returns one repository.
func (c *Client) Repository(ctx context.Context, repo Repo) (Repository, error) {
	r, _, err := c.gh.Repositories.Get(ctx, repo.Owner, repo.Name)
	if err != nil {
		return Repository{}, classify(ctx, err)
	}
	return repository(r), nil
}
