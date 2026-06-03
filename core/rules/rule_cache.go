package rules

import (
	"strings"

	"github.com/rhysd/actionlint"
)

// RuleCache checks for usage of the cache in potentially untrusted workflows.
type RuleCache struct {
	actionlint.RuleBase
	dangerousCheckoutBefore bool
	skip                    bool
	filterTriggers          []string
}

// NewRuleRunnerLabel creates new RuleRunnerLabel instance.
func NewRuleCache(filterTriggers []string) *RuleCache {
	return &RuleCache{
		RuleBase: actionlint.NewRuleBase(
			"dangerous-cache",
			"Checks for usage of actions/cache in potentially untrusted workflows (potentially exploitable if trusted workflows use cache too)",
		),
		dangerousCheckoutBefore: false,
		skip:                    false,
		filterTriggers:          filterTriggers,
	}
}

func (rule *RuleCache) VisitWorkflowPre(n *actionlint.Workflow) error {
	// check on event and set skip if needed
	rule.skip = skipAnalysis(n, rule.filterTriggers)

	return nil
}

// VisitStep is callback when visiting Step node.
func (rule *RuleCache) VisitStep(n *actionlint.Step) error {
	if rule.skip {
		return nil
	}

	e, ok := n.Exec.(*actionlint.ExecAction)
	if !ok || e.Uses == nil {
		return nil
	}

	spec := e.Uses.Value

	// search for checkout action
	dangerousCheckoutRule := NewRuleDangerousCheckout(rule.filterTriggers)
	if dangerousCheckoutRule.VisitStep(n) != nil {
		rule.dangerousCheckoutBefore = true
	}

	if strings.HasPrefix(spec, "actions/cache") && rule.dangerousCheckoutBefore {

		rule.Errorf(
			e.Inputs["path"].Value.Pos,
			"Use of action 'actions/cache' with potentially untrusted input",
		)
	}

	return nil
}

func (rule *RuleCache) VisitJobPost(job *actionlint.Job) error {
	// reset for each job
	rule.dangerousCheckoutBefore = false

	return nil
}
