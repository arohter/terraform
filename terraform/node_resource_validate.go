package terraform

import (
	"github.com/hashicorp/terraform/config/configschema"
	"github.com/hashicorp/terraform/dag"
)

// NodeValidatableResource represents a resource that is used for validation
// only.
type NodeValidatableResource struct {
	*NodeAbstractCountResource
}

// GraphNodeEvalable
func (n *NodeValidatableResource) EvalTree() EvalNode {
	// Ensure we're validating
	c := n.NodeAbstractCountResource
	c.Validate = true
	return c.EvalTree()
}

// GraphNodeDynamicExpandable
func (n *NodeValidatableResource) DynamicExpand(ctx EvalContext) (*Graph, error) {
	// Grab the state which we read
	state, lock := ctx.State()
	lock.RLock()
	defer lock.RUnlock()

	// Expand the resource count which must be available by now from EvalTree
	count := 1
	if n.Config.RawCount.Value() != unknownValue() {
		var err error
		count, err = n.Config.Count()
		if err != nil {
			return nil, err
		}
	}

	// The concrete resource factory we'll use
	concreteResource := func(a *NodeAbstractResourceInstance) dag.Vertex {
		// Add the config and state since we don't do that via transforms
		a.Config = n.Config
		a.ResolvedProvider = n.ResolvedProvider

		return &NodeValidatableResourceInstance{
			NodeAbstractResourceInstance: a,
		}
	}

	// Start creating the steps
	steps := []GraphTransformer{
		// Expand the count.
		&ResourceCountTransformer{
			Concrete: concreteResource,
			Count:    count,
			Addr:     n.ResourceAddr(),
		},

		// Attach the state
		&AttachStateTransformer{State: state},

		// Targeting
		&TargetsTransformer{ParsedTargets: n.Targets},

		// Connect references so ordering is correct
		&ReferenceTransformer{},

		// Make sure there is a single root
		&RootTransformer{},
	}

	// Build the graph
	b := &BasicGraphBuilder{
		Steps:    steps,
		Validate: true,
		Name:     "NodeValidatableResource",
	}

	graph, diags := b.Build(ctx.Path())
	return graph, diags.ErrWithWarnings()
}

// This represents a _single_ resource instance to validate.
type NodeValidatableResourceInstance struct {
	*NodeAbstractResourceInstance
}

// GraphNodeEvalable
func (n *NodeValidatableResourceInstance) EvalTree() EvalNode {
	addr := n.ResourceInstanceAddr()
	config := n.Config

	// Declare a bunch of variables that are used for state during
	// evaluation. These are written to via pointers passed to the EvalNodes
	// below.
	var provider ResourceProvider
	var providerSchema *ProviderSchema

	seq := &EvalSequence{
		Nodes: []EvalNode{
			&EvalValidateSelfRef{
				Addr:   addr.Resource,
				Config: config.Config,
			},
			&EvalGetProvider{
				Addr:   n.ResolvedProvider.ProviderConfig,
				Output: &provider,
				Schema: &providerSchema,
			},
			&EvalValidateResource{
				Addr:           addr.Resource,
				Provider:       &provider,
				ProviderSchema: &providerSchema,
				Config:         config,
			},
		},
	}

	if managed := n.Config.Managed; managed != nil {
		// Validate all the provisioners
		for _, p := range managed.Provisioners {
			var provisioner ResourceProvisioner
			var provisionerSchema *configschema.Block
			seq.Nodes = append(
				seq.Nodes,
				&EvalGetProvisioner{
					Name:   p.Type,
					Output: &provisioner,
					Schema: &provisionerSchema,
				},
				&EvalValidateProvisioner{
					Provisioner: &provisioner,
					Schema:      &provisionerSchema,
					Config:      p,
				},
			)
		}
	}

	return seq
}
