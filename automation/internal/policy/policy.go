// Package policy decides whether the broker may issue credentials for an
// action to a workflow run. It is pure: callers gather the facts (verified
// token claims, the run and runner from the GitHub API, main's HEAD, the last
// applied SHA and the merged-by chain) and Authorize returns nil or a reason.
//
// Rules (Cypherworks/lab-deploy#245):
//   - every action: the token and run agree, are on main at main's HEAD, from
//     an allowed workflow via a reusable _*.yml on main, on a runner the
//     registrar minted in the apply group;
//   - plan: push, schedule or workflow_dispatch;
//   - ansible-apply on push: every commit since the last applied SHA was
//     merged by the apply actor; no actor pin, the chain is the trust;
//   - ansible-apply on schedule: head is the last applied SHA (a
//     re-assertion, nothing new); no actor pin;
//   - terraform-apply: only workflow_dispatch where actor and triggering
//     actor are both the apply actor, plus the merged-by chain.
package policy

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
)

// Action is what the caller wants credentials for.
type Action int

const (
	Plan Action = iota
	AnsibleApply
	TerraformApply
)

func (a Action) String() string {
	switch a {
	case Plan:
		return "plan"
	case AnsibleApply:
		return "ansible-apply"
	case TerraformApply:
		return "terraform-apply"
	}
	return "unknown action " + strconv.Itoa(int(a))
}

// Config is the deploy repo's data: IDs, workflows and the runner group.
type Config struct {
	RepositoryID       string
	RepositoryOwnerID  string
	ApplyActorID       int64
	JobWorkflowPattern string
	RunnerGroup        string
	Workflows          map[Action][]string
}

// Token holds the verified OIDC claims the policy uses.
type Token struct {
	RepositoryID      string
	RepositoryOwnerID string
	Ref               string
	SHA               string
	EventName         string
	ActorID           string
	JobWorkflowRef    string
}

// Run is the workflow run as the GitHub API reports it.
type Run struct {
	Event             string
	Path              string
	HeadBranch        string
	HeadSHA           string
	ActorID           int64
	TriggeringActorID int64
}

// Runner is the calling job's runner, checked against the registrar.
type Runner struct {
	MintedByRegistrar bool
	Group             string
}

// Commit is one commit on main; MergedByID is 0 if no merged PR brought it.
type Commit struct {
	SHA        string
	MergedByID int64
}

// Chain is main's commits after the last applied SHA, oldest first.
type Chain struct {
	Commits  []Commit
	Complete bool
}

// Facts are everything Authorize decides on.
type Facts struct {
	Token       Token
	Run         Run
	Runner      Runner
	MainHead    string
	LastApplied string
	Chain       Chain
}

// Authorize returns nil if the broker may issue credentials for a.
func Authorize(cfg Config, a Action, f Facts) error {
	if a != Plan && a != AnsibleApply && a != TerraformApply {
		return errors.New(a.String())
	}
	if err := checkRun(cfg, a, f); err != nil {
		return err
	}
	switch a {
	case Plan:
		return eventIn(a, f.Run.Event, "push", "schedule", "workflow_dispatch")
	case AnsibleApply:
		return authorizeAnsibleApply(cfg, f)
	default:
		return authorizeTerraformApply(cfg, f)
	}
}

// checkRun holds the checks every action shares.
func checkRun(cfg Config, a Action, f Facts) error {
	t, r := f.Token, f.Run
	switch {
	case t.RepositoryID != cfg.RepositoryID:
		return fmt.Errorf("token repository_id %q is not %q",
			t.RepositoryID, cfg.RepositoryID)
	case t.RepositoryOwnerID != cfg.RepositoryOwnerID:
		return fmt.Errorf("token repository_owner_id %q is not %q",
			t.RepositoryOwnerID, cfg.RepositoryOwnerID)
	case t.Ref != "refs/heads/main":
		return fmt.Errorf("token ref %q is not refs/heads/main", t.Ref)
	case r.HeadBranch != "main":
		return fmt.Errorf("run head branch %q is not main", r.HeadBranch)
	case t.EventName != r.Event:
		return fmt.Errorf("token event %q is not the run's event %q",
			t.EventName, r.Event)
	case t.SHA != r.HeadSHA:
		return fmt.Errorf("token sha %q is not the run's head %q",
			t.SHA, r.HeadSHA)
	case r.HeadSHA != f.MainHead:
		return fmt.Errorf("run head %q is not main HEAD %q",
			r.HeadSHA, f.MainHead)
	case !slices.Contains(cfg.Workflows[a], r.Path):
		return fmt.Errorf("workflow %q is not allowed for %s", r.Path, a)
	case !matches(cfg.JobWorkflowPattern, t.JobWorkflowRef):
		return fmt.Errorf("job_workflow_ref %q does not match %q",
			t.JobWorkflowRef, cfg.JobWorkflowPattern)
	case !f.Runner.MintedByRegistrar:
		return errors.New("runner was not minted by the registrar")
	case f.Runner.Group != cfg.RunnerGroup:
		return fmt.Errorf("runner group %q is not %q",
			f.Runner.Group, cfg.RunnerGroup)
	}
	return nil
}

// matches uses path.Match, where * never crosses a "/", unlike IAM StringLike.
func matches(pattern, s string) bool {
	ok, err := path.Match(pattern, s)
	return err == nil && ok
}

func authorizeAnsibleApply(cfg Config, f Facts) error {
	if err := eventIn(AnsibleApply, f.Run.Event, "push", "schedule"); err != nil {
		return err
	}
	if f.Run.Event == "schedule" {
		if f.LastApplied == "" || f.Run.HeadSHA != f.LastApplied {
			return fmt.Errorf("scheduled run head %q is not the last applied %q",
				f.Run.HeadSHA, f.LastApplied)
		}
		return nil
	}
	return checkChain(cfg, f)
}

func authorizeTerraformApply(cfg Config, f Facts) error {
	err := eventIn(TerraformApply, f.Run.Event, "workflow_dispatch")
	if err != nil {
		return err
	}
	want := cfg.ApplyActorID
	switch {
	case f.Run.ActorID != want:
		return fmt.Errorf("run actor %d is not the apply actor %d",
			f.Run.ActorID, want)
	case f.Run.TriggeringActorID != want:
		return fmt.Errorf("run triggering actor %d is not the apply actor %d",
			f.Run.TriggeringActorID, want)
	case f.Token.ActorID != strconv.FormatInt(want, 10):
		return fmt.Errorf("token actor %q is not the apply actor %d",
			f.Token.ActorID, want)
	}
	return checkChain(cfg, f)
}

// checkChain: every commit after the last apply came via an actor-merged PR.
func checkChain(cfg Config, f Facts) error {
	if f.LastApplied == "" {
		return errors.New("no last applied SHA to check the chain from")
	}
	if !f.Chain.Complete {
		return errors.New("merged-by chain is incomplete")
	}
	commits := f.Chain.Commits
	if f.Run.HeadSHA != f.LastApplied &&
		(len(commits) == 0 || commits[len(commits)-1].SHA != f.Run.HeadSHA) {
		return errors.New("merged-by chain does not end at the run's head")
	}
	for _, c := range commits {
		if c.MergedByID != cfg.ApplyActorID {
			return fmt.Errorf("commit %s was not merged by the apply actor",
				c.SHA)
		}
	}
	return nil
}

func eventIn(a Action, event string, allowed ...string) error {
	if !slices.Contains(allowed, event) {
		return fmt.Errorf("event %q is not allowed for %s", event, a)
	}
	return nil
}
