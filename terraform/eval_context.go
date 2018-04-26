package terraform

import (
	"sync"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/config/configschema"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

// EvalContext is the interface that is given to eval nodes to execute.
type EvalContext interface {
	// Stopped returns a channel that is closed when evaluation is stopped
	// via Terraform.Context.Stop()
	Stopped() <-chan struct{}

	// Path is the current module path.
	Path() addrs.ModuleInstance

	// Hook is used to call hook methods. The callback is called for each
	// hook and should return the hook action to take and the error.
	Hook(func(Hook) (HookAction, error)) error

	// Input is the UIInput object for interacting with the UI.
	Input() UIInput

	// InitProvider initializes the provider with the given type and address, and
	// returns the implementation of the resource provider or an error.
	//
	// It is an error to initialize the same provider more than once.
	InitProvider(typ string, addr addrs.ProviderConfig) (ResourceProvider, error)

	// Provider gets the provider instance with the given address (already
	// initialized) or returns nil if the provider isn't initialized.
	Provider(addrs.ProviderConfig) ResourceProvider

	// ProviderSchema retrieves the schema for a particular provider, which
	// must have already be initialized with InitProvider.
	ProviderSchema(addrs.ProviderConfig) *ProviderSchema

	// CloseProvider closes provider connections that aren't needed anymore.
	CloseProvider(addrs.ProviderConfig) error

	// ConfigureProvider configures the provider with the given
	// configuration. This is a separate context call because this call
	// is used to store the provider configuration for inheritance lookups
	// with ParentProviderConfig().
	ConfigureProvider(addrs.ProviderConfig, cty.Value) tfdiags.Diagnostics

	// ProviderInput and SetProviderInput are used to configure providers
	// from user input.
	ProviderInput(addrs.ProviderConfig) map[string]cty.Value
	SetProviderInput(addrs.ProviderConfig, map[string]cty.Value)

	// InitProvisioner initializes the provisioner with the given name and
	// returns the implementation of the resource provisioner or an error.
	//
	// It is an error to initialize the same provisioner more than once.
	InitProvisioner(string) (ResourceProvisioner, error)

	// Provisioner gets the provisioner instance with the given name (already
	// initialized) or returns nil if the provisioner isn't initialized.
	Provisioner(string) ResourceProvisioner

	// CloseProvisioner closes provisioner connections that aren't needed
	// anymore.
	CloseProvisioner(string) error

	// EvaluateBlock takes the given raw configuration block and associated
	// schema and evaluates it to produce a value of an object type that
	// conforms to the implied type of the schema.
	//
	// The resource argument is optional. If given, it is the resource
	// that is currently being acted upon, accessible as the "self" object.
	//
	// The returned body is an expanded version of the given body, with any
	// "dynamic" blocks replaced with zero or more static blocks. This can be
	// used to extract correct source location information about attributes of
	// the returned object value.
	EvaluateBlock(hcl.Body, *configschema.Block, *Resource) (cty.Value, hcl.Body, tfdiags.Diagnostics)

	// EvaluateExpr takes the given HCL expression and evaluates it to produce
	// a value.
	//
	// The resource argument is optional. If given, it is the resource that
	// is currently being acted upon, accessible as the "self" object.
	EvaluateExpr(hcl.Expression, cty.Type, *Resource) (cty.Value, tfdiags.Diagnostics)

	// SetModuleCallArguments defines values for the variables of a particular
	// child module call.
	//
	// Calling this function multiple times has merging behavior, keeping any
	// previously-set keys that are not present in the new map.
	SetModuleCallArguments(addrs.ModuleCallInstance, map[string]cty.Value)

	// Diff returns the global diff as well as the lock that should
	// be used to modify that diff.
	Diff() (*Diff, *sync.RWMutex)

	// State returns the global state as well as the lock that should
	// be used to modify that state.
	State() (*State, *sync.RWMutex)
}
