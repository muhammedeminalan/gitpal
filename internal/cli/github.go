package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	gh "github.com/muhammedeminalan/gitpal/internal/core/github"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/version"
)

// githubToken finds a token: the keychain, the gh CLI when configured (or
// when the keychain has none), then GITHUB_TOKEN for CI.
func (a *app) githubToken(ctx context.Context) string {
	c, _ := a.cfg()
	if c.GitHub.Auth != config.AuthGHCLI && a.env.Secrets != nil {
		if token, err := a.env.Secrets.Get(secrets.GitHubToken); err == nil && token != "" {
			return token
		}
	}
	if a.env.LookPath != nil {
		if token, err := gh.GHCLIToken(ctx, a.env.LookPath); err == nil && token != "" {
			return token
		}
	}
	return a.env.Getenv("GITHUB_TOKEN")
}

func (a *app) githubClient(ctx context.Context) (*gh.Client, error) {
	if _, err := a.cfg(); err != nil {
		return nil, err
	}
	return gh.New(a.githubToken(ctx), gh.Options{APIURL: a.env.Getenv("GITPAL_GITHUB_API_URL")})
}

// githubHost is the host GitHub remotes live on.
func (a *app) githubHost() string {
	if host := a.env.Getenv("GITPAL_GITHUB_HOST"); host != "" {
		return host
	}
	return "github.com"
}

// githubRepo is the repository the command is about: --gh-repo, else the
// GitHub remote of the current repository (origin first).
func (a *app) githubRepo(ctx context.Context, flag string) (gh.Repo, error) {
	if flag != "" {
		r, ok := gh.ParseName(flag)
		if !ok {
			return gh.Repo{}, fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgRepoBadName, flag))
		}
		return r, nil
	}
	runner, root, err := a.git()
	if err != nil {
		return gh.Repo{}, err
	}
	remotes, err := runner.Remotes(ctx, root)
	if err != nil {
		return gh.Repo{}, err
	}
	for _, preferOrigin := range []bool{true, false} {
		for _, r := range remotes {
			if preferOrigin != (r.Name == "origin") {
				continue
			}
			if repo, ok := gh.ParseRemote(r.FetchURL, a.githubHost()); ok {
				return repo, nil
			}
		}
	}
	return gh.Repo{}, &gh.Error{Kind: gh.KindNotGitHub}
}

// warnIfProtected tells, before a push, that GitHub protects the branch
// (spec §5). It never blocks and stays silent when GitHub cannot be asked.
func (a *app) warnIfProtected(ctx context.Context, branch string) {
	if a.flags.json || a.githubToken(ctx) == "" {
		return
	}
	repo, err := a.githubRepo(ctx, "")
	if err != nil {
		return
	}
	client, err := a.githubClient(ctx)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if protected, err := client.BranchProtected(ctx, repo, branch); err == nil && protected {
		fmt.Fprintln(a.env.Stderr, a.styleFor(a.env.Stderr).yellow(i18n.T(a.lang, i18n.MsgRepoProtectedWarn, branch)))
	}
}

// fixesHint suggests `Fixes #N` when the branch names an issue and the
// message does not mention it.
func (a *app) fixesHint(ctx context.Context, message string) {
	runner, root, err := a.git()
	if err != nil || a.flags.json {
		return
	}
	branch, err := runner.CurrentBranch(ctx, root)
	if err != nil {
		return
	}
	if n, ok := gh.IssueFromBranch(branch); ok && !strings.Contains(message, "#"+strconv.Itoa(n)) {
		fmt.Fprintln(a.env.Stderr, a.styleFor(a.env.Stderr).dim(i18n.T(a.lang, i18n.MsgIssueFixesHint, n, n)))
	}
}

func (a *app) newAuthGitHubCmd() *cobra.Command {
	var useToken, useGH bool
	cmd := &cobra.Command{
		Use: "github", Short: i18n.T(a.lang, i18n.MsgAuthGitHubShort), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if _, err := a.cfg(); err != nil {
				return err
			}
			path, err := config.GlobalPath(a.cfgEnv)
			if err != nil {
				return err
			}
			var token string
			switch {
			case useGH:
				if token, err = gh.GHCLIToken(ctx, a.env.LookPath); err != nil || token == "" {
					return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgAuthGitHubNoGH))
				}
			case useToken:
				if token, err = a.readSecret(i18n.T(a.lang, i18n.MsgAuthPrompt, "GitHub")); err != nil {
					return err
				}
			default:
				if token, err = a.deviceFlow(ctx); err != nil {
					return err
				}
			}
			client, err := gh.New(token, gh.Options{APIURL: a.env.Getenv("GITPAL_GITHUB_API_URL")})
			if err != nil {
				return err
			}
			login, err := client.User(ctx)
			if err != nil {
				return err
			}
			if useGH {
				// The gh CLI keeps the token; gitpal only remembers to ask it.
				err = config.SetGlobal(path, "github.auth", string(config.AuthGHCLI))
			} else {
				err = a.env.Secrets.Set(secrets.GitHubToken, token)
				if err == nil {
					err = config.SetGlobal(path, "github.auth", string(config.AuthDeviceFlow))
				}
			}
			if err != nil {
				return err
			}
			a.say(i18n.MsgAuthGitHubDone, login)
			return nil
		},
	}
	cmd.Flags().BoolVar(&useToken, "token", false, i18n.T(a.lang, i18n.MsgAuthGitHubToken))
	cmd.Flags().BoolVar(&useGH, "gh", false, i18n.T(a.lang, i18n.MsgAuthGitHubGH))
	cmd.MarkFlagsMutuallyExclusive("token", "gh")
	return cmd
}

func (a *app) deviceFlow(ctx context.Context) (string, error) {
	clientID := a.env.Getenv("GITPAL_GITHUB_CLIENT_ID")
	if clientID == "" {
		clientID = version.GitHubClientID
	}
	if clientID == "" {
		return "", fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgAuthGitHubNoClient))
	}
	flow := gh.DeviceFlow{ClientID: clientID, WebURL: a.env.Getenv("GITPAL_GITHUB_WEB_URL")}
	code, err := flow.Start(ctx)
	if err != nil {
		return "", err
	}
	st := a.styleFor(a.env.Stderr)
	fmt.Fprintln(a.env.Stderr, i18n.T(a.lang, i18n.MsgAuthGitHubCode, code.VerificationURI, st.bold(code.UserCode)))
	fmt.Fprintln(a.env.Stderr, st.dim(i18n.T(a.lang, i18n.MsgAuthGitHubWaiting)))
	return flow.Wait(ctx, code)
}

func (a *app) newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repo", Short: i18n.T(a.lang, i18n.MsgRepoShort)}
	var n gh.NewRepository
	var push bool
	create := &cobra.Command{
		Use: "create [name]", Short: i18n.T(a.lang, i18n.MsgRepoCreateShort), Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := a.githubClient(ctx)
			if err != nil {
				return err
			}
			n.Name = filepath.Base(a.session.dir)
			if a.session.repoRoot != "" {
				n.Name = filepath.Base(a.session.repoRoot)
			}
			if len(args) == 1 {
				n.Name = args[0]
			}
			created, err := client.CreateRepository(ctx, n)
			if err != nil {
				return err
			}
			a.say(i18n.MsgRepoCreated, created.HTMLURL)
			if a.session.repoRoot == "" {
				return nil
			}
			// Connect the folder and, if asked, push it.
			runner, root := a.session.runner, a.session.repoRoot
			remotes, err := runner.Remotes(ctx, root)
			if err != nil {
				return err
			}
			for _, r := range remotes {
				if r.Name == "origin" {
					a.say(i18n.MsgRepoHasOrigin)
					return nil
				}
			}
			url := created.CloneURL
			if url == "" {
				url = "https://" + a.githubHost() + "/" + created.Repo.String() + ".git"
			}
			if err := runner.AddRemote(ctx, root, "origin", url); err != nil {
				return err
			}
			a.say(i18n.MsgRepoConnected, "origin")
			if push {
				_, err = a.push(ctx, git.PushOptions{Remote: "origin", SetUpstream: true})
			}
			return err
		},
	}
	create.Flags().BoolVar(&n.Private, "private", false, i18n.T(a.lang, i18n.MsgRepoPrivate))
	create.Flags().StringVar(&n.Description, "description", "", i18n.T(a.lang, i18n.MsgRepoDescription))
	create.Flags().BoolVar(&n.README, "readme", false, i18n.T(a.lang, i18n.MsgRepoREADME))
	create.Flags().StringVar(&n.Gitignore, "gitignore", "", i18n.T(a.lang, i18n.MsgRepoGitignore))
	create.Flags().StringVar(&n.License, "license", "", i18n.T(a.lang, i18n.MsgRepoLicense))
	create.Flags().StringVar(&n.Org, "org", "", i18n.T(a.lang, i18n.MsgRepoOrg))
	create.Flags().BoolVar(&push, "push", false, i18n.T(a.lang, i18n.MsgRepoPush))

	var limit int
	list := &cobra.Command{
		Use: "list", Short: i18n.T(a.lang, i18n.MsgRepoListShort), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.githubClient(cmd.Context())
			if err != nil {
				return err
			}
			repos, err := client.Repositories(cmd.Context(), limit)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(repos)
			}
			st := a.styleFor(a.env.Stdout)
			for _, r := range repos {
				tag := ""
				if r.Private {
					tag = " " + st.yellow("("+i18n.T(a.lang, i18n.MsgRepoPrivateTag)+")")
				}
				fmt.Fprintf(a.env.Stdout, "%s%s  %s\n", st.bold(r.Repo.String()), tag, st.dim(r.Description))
			}
			return nil
		},
	}
	list.Flags().IntVarP(&limit, "limit", "n", 30, i18n.T(a.lang, i18n.MsgLogMax))

	clone := &cobra.Command{
		Use: "clone <owner/name> [dir]", Short: i18n.T(a.lang, i18n.MsgRepoCloneShort), Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, ok := gh.ParseName(args[0])
			if !ok {
				return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgRepoBadName, args[0]))
			}
			base := a.env.Getenv("GITPAL_GITHUB_WEB_URL")
			if base == "" {
				base = "https://" + a.githubHost()
			}
			dir := repo.Name
			if len(args) == 2 {
				dir = args[1]
			}
			dir = a.target(dir)
			opts := git.CloneOptions{}
			if !a.flags.json {
				opts.Progress = a.env.Stderr
			}
			if err := a.session.runner.Clone(cmd.Context(), strings.TrimRight(base, "/")+"/"+repo.String()+".git", dir, opts); err != nil {
				return err
			}
			a.say(i18n.MsgCloneDone, dir)
			return nil
		},
	}
	cmd.AddCommand(create, list, clone)
	return cmd
}

// number reads a pull request or issue number.
func (a *app) number(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(s, "#"))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgPRBadNumber, s))
	}
	return n, nil
}

// withGitHub runs fn with the client and the repository.
func (a *app) withGitHub(repoFlag *string, fn func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client, err := a.githubClient(ctx)
		if err != nil {
			return err
		}
		repo, err := a.githubRepo(ctx, *repoFlag)
		if err != nil {
			return err
		}
		return fn(ctx, client, repo, args)
	}
}

func (a *app) newPRCmd() *cobra.Command {
	var repoFlag string
	cmd := &cobra.Command{Use: "pr", Short: i18n.T(a.lang, i18n.MsgPRShort)}
	cmd.PersistentFlags().StringVar(&repoFlag, "gh-repo", "", i18n.T(a.lang, i18n.MsgRepoCloneShort))

	var n gh.NewPullRequest
	var useAI bool
	create := &cobra.Command{
		Use: "create", Short: i18n.T(a.lang, i18n.MsgPRCreateShort), Args: cobra.NoArgs,
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, _ []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			if n.Head, err = runner.CurrentBranch(ctx, root); err != nil {
				return err
			}
			if n.Head == "" {
				return &git.Error{Kind: git.KindDetachedHead, Args: []string{"pr", "create"}, ExitCode: -1}
			}
			if n.Base == "" {
				r, err := c.Repository(ctx, repo)
				if err != nil {
					return err
				}
				n.Base = r.DefaultBranch
			}
			// The branch must be on GitHub first.
			if _, err := a.push(ctx, git.PushOptions{}); err != nil {
				return err
			}
			if err := a.prText(ctx, &n, useAI); err != nil {
				return err
			}
			pr, err := c.CreatePullRequest(ctx, repo, n)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(pr)
			}
			a.say(i18n.MsgPRCreated, pr.Number, pr.HTMLURL)
			return nil
		}),
	}
	create.Flags().StringVarP(&n.Title, "title", "t", "", i18n.T(a.lang, i18n.MsgPRTitle))
	create.Flags().StringVarP(&n.Body, "body", "b", "", i18n.T(a.lang, i18n.MsgPRBody))
	create.Flags().StringVar(&n.Base, "base", "", i18n.T(a.lang, i18n.MsgPRBase))
	create.Flags().BoolVar(&n.Draft, "draft", false, i18n.T(a.lang, i18n.MsgPRDraft))
	create.Flags().BoolVar(&useAI, "ai", false, i18n.T(a.lang, i18n.MsgPRAI))

	var state string
	var limit int
	list := &cobra.Command{
		Use: "list", Short: i18n.T(a.lang, i18n.MsgPRListShort), Args: cobra.NoArgs,
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, _ []string) error {
			prs, err := c.PullRequests(ctx, repo, state, limit)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(prs)
			}
			if len(prs) == 0 {
				a.say(i18n.MsgPRNone)
			}
			st := a.styleFor(a.env.Stdout)
			for _, p := range prs {
				fmt.Fprintf(a.env.Stdout, "%s %s  %s → %s  %s\n", st.green("#"+strconv.Itoa(p.Number)), p.Title,
					p.Head, p.Base, st.dim(p.Author))
			}
			return nil
		}),
	}
	list.Flags().StringVar(&state, "state", "open", i18n.T(a.lang, i18n.MsgPRState))
	list.Flags().IntVarP(&limit, "limit", "n", 30, i18n.T(a.lang, i18n.MsgLogMax))

	view := &cobra.Command{
		Use: "view <number>", Short: i18n.T(a.lang, i18n.MsgPRViewShort), Args: cobra.ExactArgs(1),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			p, err := c.PullRequestByNumber(ctx, repo, num)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(p)
			}
			st := a.styleFor(a.env.Stdout)
			fmt.Fprintf(a.env.Stdout, "%s %s\n%s → %s · %s · %s\n%s\n\n%s\n", st.bold("#"+strconv.Itoa(p.Number)), st.bold(p.Title),
				p.Head, p.Base, p.State, p.Author, st.dim(p.HTMLURL), p.Body)
			return nil
		}),
	}

	var squash, rebase bool
	merge := &cobra.Command{
		Use: "merge <number>", Short: i18n.T(a.lang, i18n.MsgPRMergeShort), Args: cobra.ExactArgs(1),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			method := gh.MergeCommit
			switch {
			case squash:
				method = gh.MergeSquash
			case rebase:
				method = gh.MergeRebase
			}
			p, err := c.PullRequestByNumber(ctx, repo, num)
			if err != nil {
				return err
			}
			// Merging on GitHub changes the shared repository: a "caution"
			// operation, confirmed once (or with --yes).
			if !a.flags.yes {
				if !a.interactive() {
					return ErrNeedsYes
				}
				line, err := a.readLine(i18n.T(a.lang, i18n.MsgPRMergeConfirm, num, p.Head, p.Base, method))
				if err != nil {
					return err
				}
				if !a.yes(line, false) {
					return errCanceled
				}
			}
			sha, err := c.MergePullRequest(ctx, repo, num, method)
			if err != nil {
				return err
			}
			a.say(i18n.MsgPRMerged, num, short(sha))
			return nil
		}),
	}
	merge.Flags().BoolVar(&squash, "squash", false, i18n.T(a.lang, i18n.MsgMergeSquash))
	merge.Flags().BoolVar(&rebase, "rebase", false, i18n.T(a.lang, i18n.MsgRebaseShort))
	merge.MarkFlagsMutuallyExclusive("squash", "rebase")

	closeCmd := &cobra.Command{
		Use: "close <number>", Short: i18n.T(a.lang, i18n.MsgPRCloseShort), Args: cobra.ExactArgs(1),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			if err := c.ClosePullRequest(ctx, repo, num); err != nil {
				return err
			}
			a.say(i18n.MsgPRClosed, num)
			return nil
		}),
	}
	checkout := &cobra.Command{
		Use: "checkout <number>", Short: i18n.T(a.lang, i18n.MsgPRCheckoutShort), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			branch := "pr-" + strconv.Itoa(num)
			if err := runner.FetchRef(ctx, root, "origin", "refs/pull/"+strconv.Itoa(num)+"/head", branch); err != nil {
				return err
			}
			if _, err := a.runOp(ctx, safetySwitch(branch)); err != nil {
				return err
			}
			a.say(i18n.MsgBranchSwitched, branch)
			return nil
		},
	}
	var body string
	comment := &cobra.Command{
		Use: "comment <number>", Short: i18n.T(a.lang, i18n.MsgPRCommentShort), Args: cobra.ExactArgs(1),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			if strings.TrimSpace(body) == "" {
				return &git.ArgError{Code: git.ArgEmptyMessage}
			}
			if err := c.Comment(ctx, repo, num, body); err != nil {
				return err
			}
			a.say(i18n.MsgPRCommented)
			return nil
		}),
	}
	comment.Flags().StringVarP(&body, "message", "m", "", i18n.T(a.lang, i18n.MsgPRBody))

	cmd.AddCommand(create, list, view, merge, closeCmd, checkout, comment)
	return cmd
}

// prText fills in a missing title and description: from the AI when asked
// (or set up), else the single commit's subject or what the user types.
func (a *app) prText(ctx context.Context, n *gh.NewPullRequest, useAI bool) error {
	if n.Title != "" {
		return nil
	}
	runner, root, err := a.git()
	if err != nil {
		return err
	}
	commits, err := runner.Log(ctx, root, git.LogOptions{Revs: []string{n.Base + "..HEAD"}, NoMerges: true})
	if err != nil {
		return err
	}
	if useAI || a.aiByDefault() {
		if provider, c, err := a.aiProvider(); err == nil {
			if ok, err := a.aiConsent(c, provider.Name()); err == nil && ok {
				files, err := runner.Diff(ctx, root, git.DiffOptions{Since: n.Base})
				if err != nil {
					return err
				}
				var subjects []string
				for _, cm := range commits {
					subjects = append(subjects, cm.Subject)
				}
				text, err := provider.GeneratePRDescription(ctx, ai.PRInput{
					Payload: ai.Prepare(files, 0), Commits: subjects, Branch: n.Head, Base: n.Base, Language: c.Commit.MessageLanguage,
				})
				if err == nil {
					n.Title, n.Body = text.Title, text.Body
					return nil
				}
				if _, ferr := a.aiFallback(err, useAI); ferr != nil {
					return ferr
				}
			}
		} else if useAI {
			if _, ferr := a.aiFallback(err, true); ferr != nil {
				return ferr
			}
		}
	}
	if len(commits) == 1 {
		n.Title = commits[0].Subject
		if n.Body == "" {
			n.Body = commits[0].Body
		}
		return nil
	}
	if !a.interactive() {
		return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgNeedsTerminal, "gitpal pr create --title ..."))
	}
	for n.Title == "" {
		line, err := a.readLine(i18n.T(a.lang, i18n.MsgPRTitlePrompt))
		if err != nil {
			return err
		}
		n.Title = strings.TrimSpace(line)
	}
	return nil
}

func (a *app) newIssueCmd() *cobra.Command {
	var repoFlag string
	cmd := &cobra.Command{Use: "issue", Short: i18n.T(a.lang, i18n.MsgIssueShort)}
	cmd.PersistentFlags().StringVar(&repoFlag, "gh-repo", "", i18n.T(a.lang, i18n.MsgRepoCloneShort))
	var state string
	var limit int
	list := &cobra.Command{
		Use: "list", Short: i18n.T(a.lang, i18n.MsgIssueListShort), Args: cobra.NoArgs,
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, _ []string) error {
			issues, err := c.Issues(ctx, repo, state, limit)
			if err != nil {
				return err
			}
			if a.flags.json {
				if issues == nil {
					issues = []gh.Issue{}
				}
				return a.writeJSON(issues)
			}
			if len(issues) == 0 {
				a.say(i18n.MsgIssueNone)
			}
			st := a.styleFor(a.env.Stdout)
			for _, i := range issues {
				labels := ""
				if len(i.Labels) > 0 {
					labels = " " + st.yellow("["+strings.Join(i.Labels, ", ")+"]")
				}
				fmt.Fprintf(a.env.Stdout, "%s %s%s\n", st.green("#"+strconv.Itoa(i.Number)), i.Title, labels)
			}
			return nil
		}),
	}
	list.Flags().StringVar(&state, "state", "open", i18n.T(a.lang, i18n.MsgPRState))
	list.Flags().IntVarP(&limit, "limit", "n", 30, i18n.T(a.lang, i18n.MsgLogMax))

	var title, body string
	var labels []string
	create := &cobra.Command{
		Use: "create", Short: i18n.T(a.lang, i18n.MsgIssueCreateShort), Args: cobra.NoArgs,
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, _ []string) error {
			for strings.TrimSpace(title) == "" {
				if !a.interactive() {
					return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgNeedsTerminal, "gitpal issue create --title ..."))
				}
				line, err := a.readLine(i18n.T(a.lang, i18n.MsgPRTitlePrompt))
				if err != nil {
					return err
				}
				title = line
			}
			i, err := c.CreateIssue(ctx, repo, strings.TrimSpace(title), body, labels)
			if err != nil {
				return err
			}
			a.say(i18n.MsgIssueCreated, i.Number, i.HTMLURL)
			return nil
		}),
	}
	create.Flags().StringVarP(&title, "title", "t", "", i18n.T(a.lang, i18n.MsgPRTitle))
	create.Flags().StringVarP(&body, "body", "b", "", i18n.T(a.lang, i18n.MsgPRBody))
	create.Flags().StringSliceVarP(&labels, "label", "l", nil, i18n.T(a.lang, i18n.MsgIssueLabels))

	closeCmd := &cobra.Command{
		Use: "close <number>", Short: i18n.T(a.lang, i18n.MsgIssueCloseShort), Args: cobra.ExactArgs(1),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			if err := c.CloseIssue(ctx, repo, num); err != nil {
				return err
			}
			a.say(i18n.MsgIssueClosed, num)
			return nil
		}),
	}
	label := &cobra.Command{
		Use: "label <number> <labels...>", Short: i18n.T(a.lang, i18n.MsgIssueLabelShort), Args: cobra.MinimumNArgs(2),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			num, err := a.number(args[0])
			if err != nil {
				return err
			}
			if err := c.LabelIssue(ctx, repo, num, args[1:]); err != nil {
				return err
			}
			a.say(i18n.MsgIssueLabeled, num)
			return nil
		}),
	}
	cmd.AddCommand(list, create, closeCmd, label)
	return cmd
}

func (a *app) newReleaseCmd() *cobra.Command {
	var repoFlag string
	var n gh.NewRelease
	cmd := &cobra.Command{Use: "release", Short: i18n.T(a.lang, i18n.MsgReleaseShort)}
	cmd.PersistentFlags().StringVar(&repoFlag, "gh-repo", "", i18n.T(a.lang, i18n.MsgRepoCloneShort))
	create := &cobra.Command{
		Use: "create <tag>", Short: i18n.T(a.lang, i18n.MsgReleaseCreateShort), Args: cobra.ExactArgs(1),
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, args []string) error {
			n.Tag = args[0]
			url, err := c.CreateRelease(ctx, repo, n)
			if err != nil {
				return err
			}
			a.say(i18n.MsgReleaseCreated, url)
			return nil
		}),
	}
	create.Flags().StringVarP(&n.Name, "title", "t", "", i18n.T(a.lang, i18n.MsgPRTitle))
	create.Flags().StringVar(&n.Body, "notes", "", i18n.T(a.lang, i18n.MsgReleaseNotes))
	create.Flags().BoolVar(&n.GenerateNotes, "generate-notes", false, i18n.T(a.lang, i18n.MsgReleaseGenerate))
	create.Flags().BoolVar(&n.Draft, "draft", false, i18n.T(a.lang, i18n.MsgPRDraft))
	create.Flags().BoolVar(&n.Prerelease, "prerelease", false, i18n.T(a.lang, i18n.MsgReleasePrerelease))
	cmd.AddCommand(create)
	return cmd
}

func (a *app) newActionsCmd() *cobra.Command {
	var repoFlag, branch string
	var limit int
	cmd := &cobra.Command{
		Use: "actions", Short: i18n.T(a.lang, i18n.MsgActionsShort), Args: cobra.NoArgs,
		RunE: a.withGitHub(&repoFlag, func(ctx context.Context, c *gh.Client, repo gh.Repo, _ []string) error {
			if branch == "" && a.session.repoRoot != "" {
				branch, _ = a.session.runner.CurrentBranch(ctx, a.session.repoRoot)
			}
			runs, err := c.WorkflowRuns(ctx, repo, branch, limit)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(runs)
			}
			if len(runs) == 0 {
				a.say(i18n.MsgActionsNone)
			}
			st := a.styleFor(a.env.Stdout)
			for _, r := range runs {
				result := r.Conclusion
				switch r.Conclusion {
				case "success":
					result = st.green("✓ " + result)
				case "failure", "cancelled", "timed_out":
					result = st.red("✗ " + result)
				case "":
					result = st.yellow("… " + r.Status)
				}
				fmt.Fprintf(a.env.Stdout, "%s  %s  %s  %s\n", result, r.Workflow, short(r.Commit), st.dim(r.HTMLURL))
			}
			return nil
		}),
	}
	cmd.Flags().StringVar(&repoFlag, "gh-repo", "", i18n.T(a.lang, i18n.MsgRepoCloneShort))
	cmd.Flags().StringVar(&branch, "branch", "", i18n.T(a.lang, i18n.MsgBranchShort))
	cmd.Flags().IntVarP(&limit, "limit", "n", 5, i18n.T(a.lang, i18n.MsgLogMax))
	return cmd
}
