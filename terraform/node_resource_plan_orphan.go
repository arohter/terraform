package terraform

// NodePlannableResourceOrphan represents a resource that is "applyable":
// it is ready to be applied and is represented by a diff.
type NodePlannableResourceInstanceOrphan struct {
	*NodeAbstractResourceInstance
}

var (
	_ GraphNodeEvalable = (*NodePlannableResourceInstanceOrphan)(nil)
)

func (n *NodePlannableResourceInstanceOrphan) Name() string {
	return n.ResourceInstanceAddr().String() + " (orphan)"
}

// GraphNodeEvalable
func (n *NodePlannableResourceInstanceOrphan) EvalTree() EvalNode {
	addr := n.NodeAbstractResource.Addr

	// stateId is the ID to put into the state
	stateId := addr.stateId()

	// Build the instance info. More of this will be populated during eval
	info := &InstanceInfo{
		Id:         stateId,
		Type:       addr.Type,
		ModulePath: normalizeModulePath(addr.Path),
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
				Resource:   n.Config,
				ResourceId: stateId,
				Diff:       &diff,
			},
			&EvalWriteDiff{
				Name: stateId,
				Diff: &diff,
			},
		},
	}
}
