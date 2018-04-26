package terraform

import (
	"github.com/hashicorp/terraform/addrs"
)

// NodePlanDestroyableResource represents a resource that is "applyable":
// it is ready to be applied and is represented by a diff.
type NodePlanDestroyableResource struct {
	*NodeAbstractResourceInstance
}

var (
	_ GraphNodeDestroyer = (*NodePlanDestroyableResource)(nil)
	_ GraphNodeEvalable  = (*NodePlanDestroyableResource)(nil)
)

// GraphNodeDestroyer
func (n *NodePlanDestroyableResource) DestroyAddr() *addrs.AbsResourceInstance {
	addr := n.ResourceInstanceAddr()
	return &addr
}

// GraphNodeEvalable
func (n *NodePlanDestroyableResource) EvalTree() EvalNode {
	addr := n.ResourceInstanceAddr

	// stateId is the ID to put into the state
	stateId := addr.stateId() // TODO: convert to legacy ResourceAddress to get this method

	// Build the instance info. More of this will be populated during eval
	info := &InstanceInfo{
		Id:   stateId,
		Type: addr.Type,
	}

	// Declare a bunch of variables that are used for state during
	// evaluation. Most of this are written to by-address below.
	var diff *InstanceDiff
	var state *InstanceState

	return &EvalSequence{
		Nodes: []EvalNode{
			&EvalReadState{
				Name:   stateId,
				Output: &state,
			},
			&EvalDiffDestroy{
				Info:   info,
				State:  &state,
				Output: &diff,
			},
			&EvalCheckPreventDestroy{
				Resource: n.Config,
				Diff:     &diff,
			},
			&EvalWriteDiff{
				Name: stateId,
				Diff: &diff,
			},
		},
	}
}
