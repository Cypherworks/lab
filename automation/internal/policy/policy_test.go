package policy

import (
	"strings"
	"testing"
)

const (
	lloyd  = int64(2228673)
	bot    = int64(41898282)
	head   = "cccccccccccccccccccccccccccccccccccccccc"
	middle = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	last   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func config() Config {
	return Config{
		RepositoryID:       "1277716830",
		RepositoryOwnerID:  "206194062",
		ApplyActorID:       lloyd,
		JobWorkflowPattern: "Cypherworks/lab-deploy/.github/workflows/_*.yml@refs/heads/main",
		RunnerGroup:        "lab-apply",
		Workflows: map[Action][]string{
			Plan:           {".github/workflows/plan.yml", ".github/workflows/reconcile.yml"},
			AnsibleApply:   {".github/workflows/ansible.yml", ".github/workflows/reconcile.yml"},
			TerraformApply: {".github/workflows/apply.yml"},
		},
	}
}

// facts is a consistent lloyd run on main with a lloyd-merged 2-commit chain.
func facts(event, path string) Facts {
	return Facts{
		Token: Token{
			RepositoryID:      "1277716830",
			RepositoryOwnerID: "206194062",
			Ref:               "refs/heads/main",
			SHA:               head,
			EventName:         event,
			ActorID:           "2228673",
			JobWorkflowRef:    "Cypherworks/lab-deploy/.github/workflows/_ansible.yml@refs/heads/main",
		},
		Run: Run{
			Event:             event,
			Path:              path,
			HeadBranch:        "main",
			HeadSHA:           head,
			ActorID:           lloyd,
			TriggeringActorID: lloyd,
		},
		Runner:      Runner{MintedByRegistrar: true, Group: "lab-apply"},
		MainHead:    head,
		LastApplied: last,
		Chain: Chain{
			Complete: true,
			Commits: []Commit{
				{SHA: middle, MergedByID: lloyd},
				{SHA: head, MergedByID: lloyd},
			},
		},
	}
}

func allow(t *testing.T, a Action, f Facts) {
	t.Helper()
	if err := Authorize(config(), a, f); err != nil {
		t.Fatalf("%s denied: %v", a, err)
	}
}

func deny(t *testing.T, a Action, f Facts, why string) {
	t.Helper()
	err := Authorize(config(), a, f)
	if err == nil {
		t.Fatalf("%s allowed, want denied (%s)", a, why)
	}
	if !strings.Contains(err.Error(), why) {
		t.Fatalf("%s denied for %q, want a reason containing %q", a, err, why)
	}
}

// Ansible apply on push: no actor pin, but the merged-by chain is mandatory.

func TestAnsibleApplyOnPushWithLloydMergedChain(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.Run.ActorID, f.Run.TriggeringActorID, f.Token.ActorID = bot, bot, "41898282"
	allow(t, AnsibleApply, f)
}

func TestAnsibleApplyOnPushDeniedWithBotMergedCommit(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.Chain.Commits[0].MergedByID = bot
	deny(t, AnsibleApply, f, "not merged by the apply actor")
}

func TestAnsibleApplyOnPushDeniedWithDirectPush(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.Chain.Commits[1].MergedByID = 0
	deny(t, AnsibleApply, f, "not merged by the apply actor")
}

func TestAnsibleApplyOnPushDeniedWithIncompleteChain(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.Chain.Complete = false
	deny(t, AnsibleApply, f, "chain is incomplete")
}

func TestAnsibleApplyOnPushDeniedWithoutAppliedBaseline(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.LastApplied = ""
	deny(t, AnsibleApply, f, "no last applied")
}

func TestAnsibleApplyOnPushDeniedWhenChainDoesNotEndAtHead(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.Chain.Commits = f.Chain.Commits[:1]
	deny(t, AnsibleApply, f, "does not end at the run's head")
}

func TestAnsibleApplyOnPushDeniedWithEmptyChainAheadOfBaseline(t *testing.T) {
	f := facts("push", ".github/workflows/ansible.yml")
	f.Chain.Commits = nil
	deny(t, AnsibleApply, f, "does not end at the run's head")
}

// Ansible apply on schedule: a re-assertion of what's already applied.

func TestAnsibleApplyOnScheduleAtLastApplied(t *testing.T) {
	f := facts("schedule", ".github/workflows/reconcile.yml")
	f.LastApplied = head
	f.Chain = Chain{}
	f.Run.ActorID, f.Run.TriggeringActorID, f.Token.ActorID = bot, bot, "41898282"
	allow(t, AnsibleApply, f)
}

func TestAnsibleApplyOnScheduleDeniedWhenHeadAheadOfLastApplied(t *testing.T) {
	f := facts("schedule", ".github/workflows/reconcile.yml")
	deny(t, AnsibleApply, f, "is not the last applied")
}

func TestAnsibleApplyOnDispatchDenied(t *testing.T) {
	f := facts("workflow_dispatch", ".github/workflows/ansible.yml")
	deny(t, AnsibleApply, f, "not allowed for ansible-apply")
}

// Terraform apply: only Lloyd's own workflow_dispatch.

func TestTerraformApplyOnLloydDispatch(t *testing.T) {
	allow(t, TerraformApply, facts("workflow_dispatch", ".github/workflows/apply.yml"))
}

func TestTerraformApplyOnPushDenied(t *testing.T) {
	deny(t, TerraformApply, facts("push", ".github/workflows/apply.yml"), "not allowed for terraform-apply")
}

func TestTerraformApplyOnScheduleDenied(t *testing.T) {
	deny(t, TerraformApply, facts("schedule", ".github/workflows/apply.yml"), "not allowed for terraform-apply")
}

func TestTerraformApplyDeniedForBotRerunOfLloydDispatch(t *testing.T) {
	f := facts("workflow_dispatch", ".github/workflows/apply.yml")
	f.Run.TriggeringActorID = bot
	deny(t, TerraformApply, f, "triggering actor")
}

func TestTerraformApplyDeniedForBotDispatch(t *testing.T) {
	f := facts("workflow_dispatch", ".github/workflows/apply.yml")
	f.Run.ActorID, f.Run.TriggeringActorID, f.Token.ActorID = bot, bot, "41898282"
	deny(t, TerraformApply, f, "actor")
}

func TestTerraformApplyDeniedWhenTokenActorDisagreesWithRun(t *testing.T) {
	f := facts("workflow_dispatch", ".github/workflows/apply.yml")
	f.Token.ActorID = "41898282"
	deny(t, TerraformApply, f, "token actor")
}

func TestTerraformApplyDeniedWithBotMergedCommit(t *testing.T) {
	f := facts("workflow_dispatch", ".github/workflows/apply.yml")
	f.Chain.Commits[1].MergedByID = bot
	deny(t, TerraformApply, f, "not merged by the apply actor")
}

// Plan: push, schedule and dispatch on main.

func TestPlanAllowedOnMainEvents(t *testing.T) {
	for _, event := range []string{"push", "schedule", "workflow_dispatch"} {
		t.Run(event, func(t *testing.T) {
			f := facts(event, ".github/workflows/plan.yml")
			f.Run.ActorID, f.Run.TriggeringActorID, f.Token.ActorID = bot, bot, "41898282"
			f.Chain = Chain{}
			allow(t, Plan, f)
		})
	}
}

// Checks that hold for every action.

func TestEveryActionDeniesOtherEvents(t *testing.T) {
	for _, a := range []Action{Plan, AnsibleApply, TerraformApply} {
		for _, event := range []string{"pull_request", "pull_request_target", "workflow_run", "issue_comment"} {
			t.Run(a.String()+"/"+event, func(t *testing.T) {
				f := facts(event, config().Workflows[a][0])
				deny(t, a, f, "not allowed for "+a.String())
			})
		}
	}
}

func TestEveryActionDeniesInconsistentOrForeignRuns(t *testing.T) {
	cases := map[string]struct {
		mutate func(f *Facts)
		why    string
	}{
		"other repository":       {func(f *Facts) { f.Token.RepositoryID = "1" }, "repository_id"},
		"other owner":            {func(f *Facts) { f.Token.RepositoryOwnerID = "1" }, "repository_owner_id"},
		"token not on main":      {func(f *Facts) { f.Token.Ref = "refs/heads/feature" }, "ref"},
		"run not on main":        {func(f *Facts) { f.Run.HeadBranch = "feature" }, "head branch"},
		"token/run event differ": {func(f *Facts) { f.Token.EventName = "schedule" }, "event"},
		"token/run sha differ":   {func(f *Facts) { f.Token.SHA = middle }, "sha"},
		"head behind main":       {func(f *Facts) { f.MainHead = "dddddddddddddddddddddddddddddddddddddddd" }, "main HEAD"},
		"workflow not allowed":   {func(f *Facts) { f.Run.Path = ".github/workflows/evil.yml" }, "workflow"},
		"not a reusable workflow": {func(f *Facts) {
			f.Token.JobWorkflowRef = "Cypherworks/lab-deploy/.github/workflows/plan.yml@refs/heads/main"
		}, "job_workflow_ref"},
		"reusable workflow off main": {func(f *Facts) {
			f.Token.JobWorkflowRef = "Cypherworks/lab-deploy/.github/workflows/_tf.yml@refs/heads/x"
		}, "job_workflow_ref"},
		"ref smuggled via branch": {func(f *Facts) {
			f.Token.JobWorkflowRef = "Cypherworks/lab-deploy/.github/workflows/_a.yml@refs/heads/x.yml@refs/heads/main"
		}, "job_workflow_ref"},
		"runner not minted":  {func(f *Facts) { f.Runner.MintedByRegistrar = false }, "runner"},
		"runner other group": {func(f *Facts) { f.Runner.Group = "Default" }, "runner"},
	}
	for _, a := range []Action{Plan, AnsibleApply, TerraformApply} {
		event := "push"
		if a == TerraformApply {
			event = "workflow_dispatch"
		}
		for name, tc := range cases {
			t.Run(a.String()+"/"+name, func(t *testing.T) {
				f := facts(event, config().Workflows[a][0])
				tc.mutate(&f)
				deny(t, a, f, tc.why)
			})
		}
	}
}

func TestUnknownActionDenied(t *testing.T) {
	deny(t, Action(99), facts("push", ".github/workflows/plan.yml"), "unknown action")
}
