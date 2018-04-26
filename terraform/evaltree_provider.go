package terraform

import (
	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/terraform/addrs"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/configs"
)

// ProviderEvalTree returns the evaluation tree for initializing and
// configuring providers.
func ProviderEvalTree(n *NodeApplyableProvider, config *configs.Provider) EvalNode {
	var provider ResourceProvider
	var resourceConfig *ResourceConfig
	var configBody hcl.Body
	var configVal cty.Value

	addr := n.Addr
	relAddr := addr.ProviderConfig
	configBody = config.Config

	seq := make([]EvalNode, 0, 5)
	seq = append(seq, &EvalInitProvider{
		TypeName: relAddr.Type,
		Addr:     addr.ProviderConfig,
	})

	// Input stuff
	seq = append(seq, &EvalOpFilter{
		Ops: []walkOperation{walkInput, walkImport},
		Node: &EvalSequence{
			Nodes: []EvalNode{
				&EvalGetProvider{
					Addr:   relAddr,
					Output: &provider,
				},
				&EvalInputProvider{
					Addr:     relAddr,
					Provider: &provider,
					Config:   config,
				},
			},
		},
	})

	seq = append(seq, &EvalOpFilter{
		Ops: []walkOperation{walkValidate},
		Node: &EvalSequence{
			Nodes: []EvalNode{
				&EvalGetProvider{
					Addr:   relAddr,
					Output: &provider,
				},
				&EvalValidateProvider{
					Addr:     relAddr,
					Provider: &provider,
					Config:   config,
				},
			},
		},
	})

	// Apply stuff
	seq = append(seq, &EvalOpFilter{
		Ops: []walkOperation{walkRefresh, walkPlan, walkApply, walkDestroy, walkImport},
		Node: &EvalSequence{
			Nodes: []EvalNode{
				&EvalGetProvider{
					Addr:   relAddr,
					Output: &provider,
				},
			},
		},
	})

	// We configure on everything but validate, since validate may
	// not have access to all the variables.
	seq = append(seq, &EvalOpFilter{
		Ops: []walkOperation{walkRefresh, walkPlan, walkApply, walkDestroy, walkImport},
		Node: &EvalSequence{
			Nodes: []EvalNode{
				&EvalConfigProvider{
					Addr:     relAddr,
					Provider: &provider,
					Config:   config,
				},
			},
		},
	})

	return &EvalSequence{Nodes: seq}
}

// CloseProviderEvalTree returns the evaluation tree for closing
// provider connections that aren't needed anymore.
func CloseProviderEvalTree(addr addrs.AbsProviderConfig) EvalNode {
	return &EvalCloseProvider{Addr: addr.ProviderConfig}
}
