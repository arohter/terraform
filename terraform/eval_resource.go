package terraform

import "github.com/hashicorp/terraform/addrs"

// EvalInstanceInfo is an EvalNode implementation that fills in the
// InstanceInfo as much as it can.
type EvalInstanceInfo struct {
	Info *InstanceInfo
}

// TODO: test
func (n *EvalInstanceInfo) Eval(ctx EvalContext) (interface{}, error) {
	// This old InstanceInfo type still expects a simple []string for a path,
	// and we can't updated it because it's currently used in the provider
	// plugin protocol. Therefore we flatten it here, but this means we can't
	// support instance keys in this codepath. In practice that's okay for now
	// since we don't support count/for_each on modules yet.
	// FIXME: Remove InstanceInfo altogether as part of updating the provider
	// API, and drop this EvalNode implementation along with it.

	path := ctx.Path()
	legacyPath := make([]string, len(path))
	for i, step := range path {
		if step.InstanceKey != addrs.NoKey {
			panic("EvalInstanceInfo does not support module paths with keyed instances")
		}
		legacyPath[i] = step.Name
	}

	n.Info.ModulePath = legacyPath
	return nil, nil
}
