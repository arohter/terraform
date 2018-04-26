package terraform

import (
	"fmt"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/configs"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/mitchellh/mapstructure"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// EvalValidateCount is an EvalNode implementation that validates
// the count of a resource.
type EvalValidateCount struct {
	Resource *configs.Resource
}

// TODO: test
func (n *EvalValidateCount) Eval(ctx EvalContext) (interface{}, error) {
	var diags tfdiags.Diagnostics
	var count int
	var err error

	val, valDiags := ctx.EvaluateExpr(n.Resource.Count, cty.Number, nil)
	diags = diags.Append(valDiags)
	if valDiags.HasErrors() {
		goto RETURN
	}
	if val.IsNull() || !val.IsKnown() {
		goto RETURN
	}

	err = gocty.FromCtyValue(val, &count)
	if err != nil {
		// The EvaluateExpr call above already guaranteed us a number value,
		// so if we end up here then we have something that is out of range
		// for an int, and the error message will include a description of
		// the valid range.
		rawVal := val.AsBigFloat()
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid count value",
			Detail:   fmt.Sprintf("The number %s is not a valid count value: %s.", rawVal, err),
			Subject:  n.Resource.Count.Range().Ptr(),
		})
	} else if count < 0 {
		rawVal := val.AsBigFloat()
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid count value",
			Detail:   fmt.Sprintf("The number %s is not a valid count value: count must not be negative.", rawVal),
			Subject:  n.Resource.Count.Range().Ptr(),
		})
	}

RETURN:
	return nil, diags.NonFatalErr()
}

// EvalValidateProvider is an EvalNode implementation that validates
// a provider configuration.
type EvalValidateProvider struct {
	Addr     addrs.ProviderConfig
	Provider *ResourceProvider
	Config   *configs.Provider
}

func (n *EvalValidateProvider) Eval(ctx EvalContext) (interface{}, error) {
	var diags tfdiags.Diagnostics
	provider := *n.Provider
	config := *n.Config

	schema, err := provider.GetSchema(&ProviderSchemaRequest{})
	if err != nil {
		diags = diags.Append(err)
		return nil, diags.NonFatalErr()
	}

	configSchema := schema.Provider
	configBody := buildProviderConfig(ctx, n.Addr, config.Config)
	configVal, configBody, evalDiags := ctx.EvaluateBlock(configBody, configSchema, nil)
	diags = diags.Append(evalDiags)
	if evalDiags.HasErrors() {
		return nil, diags.NonFatalErr()
	}

	// The provider API expects our legacy ResourceConfig type, so we'll need
	// to shim here.
	rc := NewResourceConfigShimmed(configVal, configSchema)

	warns, errs := provider.Validate(rc)
	if len(warns) == 0 && len(errs) == 0 {
		return nil, nil
	}

	// FIXME: Once provider.Validate itself returns diagnostics, just
	// return diags.NonFatalErr() immediately here.
	for _, warn := range warns {
		diags = diags.Append(tfdiags.SimpleWarning(warn))
	}
	for _, err := range errs {
		diags = diags.Append(err)
	}

	return nil, diags.NonFatalErr()
}

// EvalValidateProvisioner is an EvalNode implementation that validates
// the configuration of a provisioner belonging to a resource.
type EvalValidateProvisioner struct {
	Provisioner *ResourceProvisioner
	Config      **ResourceConfig
	ConnConfig  **ResourceConfig
}

func (n *EvalValidateProvisioner) Eval(ctx EvalContext) (interface{}, error) {
	provisioner := *n.Provisioner
	config := *n.Config
	var warns []string
	var errs []error

	{
		// Validate the provisioner's own config first
		w, e := provisioner.Validate(config)
		warns = append(warns, w...)
		errs = append(errs, e...)
	}

	{
		// Now validate the connection config, which might either be from
		// the provisioner block itself or inherited from the resource's
		// shared connection info.
		w, e := n.validateConnConfig(*n.ConnConfig)
		warns = append(warns, w...)
		errs = append(errs, e...)
	}

	// FIXME: Once the above functions themselves return diagnostics, just
	// return diags.NonFatalErr() immediately here.
	var diags tfdiags.Diagnostics
	for _, warn := range warns {
		diags = diags.Append(tfdiags.SimpleWarning(warn))
	}
	for _, err := range errs {
		diags = diags.Append(err)
	}

	return nil, diags.NonFatalErr()
}

func (n *EvalValidateProvisioner) validateConnConfig(connConfig *ResourceConfig) (warns []string, errs []error) {
	// We can't comprehensively validate the connection config since its
	// final structure is decided by the communicator and we can't instantiate
	// that until we have a complete instance state. However, we *can* catch
	// configuration keys that are not valid for *any* communicator, catching
	// typos early rather than waiting until we actually try to run one of
	// the resource's provisioners.

	type connConfigSuperset struct {
		// All attribute types are interface{} here because at this point we
		// may still have unresolved interpolation expressions, which will
		// appear as strings regardless of the final goal type.

		Type       interface{} `mapstructure:"type"`
		User       interface{} `mapstructure:"user"`
		Password   interface{} `mapstructure:"password"`
		Host       interface{} `mapstructure:"host"`
		Port       interface{} `mapstructure:"port"`
		Timeout    interface{} `mapstructure:"timeout"`
		ScriptPath interface{} `mapstructure:"script_path"`

		// For type=ssh only (enforced in ssh communicator)
		PrivateKey        interface{} `mapstructure:"private_key"`
		HostKey           interface{} `mapstructure:"host_key"`
		Agent             interface{} `mapstructure:"agent"`
		BastionHost       interface{} `mapstructure:"bastion_host"`
		BastionHostKey    interface{} `mapstructure:"bastion_host_key"`
		BastionPort       interface{} `mapstructure:"bastion_port"`
		BastionUser       interface{} `mapstructure:"bastion_user"`
		BastionPassword   interface{} `mapstructure:"bastion_password"`
		BastionPrivateKey interface{} `mapstructure:"bastion_private_key"`
		AgentIdentity     interface{} `mapstructure:"agent_identity"`

		// For type=winrm only (enforced in winrm communicator)
		HTTPS    interface{} `mapstructure:"https"`
		Insecure interface{} `mapstructure:"insecure"`
		CACert   interface{} `mapstructure:"cacert"`
	}

	var metadata mapstructure.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &metadata,
		Result:   &connConfigSuperset{}, // result is disregarded; we only care about unused keys
	})
	if err != nil {
		// should never happen
		errs = append(errs, err)
		return
	}

	if err := decoder.Decode(connConfig.Config); err != nil {
		errs = append(errs, err)
		return
	}

	for _, attrName := range metadata.Unused {
		errs = append(errs, fmt.Errorf("unknown 'connection' argument %q", attrName))
	}
	return
}

// EvalValidateResource is an EvalNode implementation that validates
// the configuration of a resource.
type EvalValidateResource struct {
	Addr           addrs.ResourceInstance
	Provider       *ResourceProvider
	ProviderSchema **ProviderSchema
	Config         *configs.Resource

	// IgnoreWarnings means that warnings will not be passed through. This allows
	// "just-in-time" passes of validation to continue execution through warnings.
	IgnoreWarnings bool
}

func (n *EvalValidateResource) Eval(ctx EvalContext) (interface{}, error) {
	var diags tfdiags.Diagnostics
	provider := *n.Provider
	cfg := *n.Config
	schema := *n.ProviderSchema
	mode := cfg.Mode

	var warns []string
	var errs []error

	// Provider entry point varies depending on resource mode, because
	// managed resources and data resources are two distinct concepts
	// in the provider abstraction.
	switch mode {
	case addrs.ManagedResourceMode:
		schema, exists := schema.ResourceTypes[cfg.Type]
		if !exists {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid resource type",
				Detail:   fmt.Sprintf("The provider %s does not support resource type %q.", cfg.ProviderConfigAddr(), cfg.Type),
				Subject:  &cfg.TypeRange,
			})
			return nil, diags.Err()
		}

		configVal, expBody, valDiags := ctx.EvaluateBlock(cfg.Config, schema, nil)
		diags = diags.Append(valDiags)
		if valDiags.HasErrors() {
			return nil, diags.Err()
		}

		// The provider API still expects our legacy types, so we must do some
		// shimming here.
		legacyCfg := NewResourceConfigShimmed(configVal, schema)
		warns, errs = provider.ValidateResource(cfg.Type, legacyCfg)

	case addrs.DataResourceMode:
		schema, exists := schema.DataSources[cfg.Type]
		if !exists {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid data source",
				Detail:   fmt.Sprintf("The provider %s does not support data source %q.", cfg.ProviderConfigAddr(), cfg.Type),
				Subject:  &cfg.TypeRange,
			})
			return nil, diags.Err()
		}

		configVal, expBody, valDiags := ctx.EvaluateBlock(cfg.Config, schema, nil)
		diags = diags.Append(valDiags)
		if valDiags.HasErrors() {
			return nil, diags.Err()
		}

		// The provider API still expects our legacy types, so we must do some
		// shimming here.
		legacyCfg := NewResourceConfigShimmed(configVal, schema)
		warns, errs = provider.ValidateDataSource(cfg.Type, legacyCfg)
	}

	// FIXME: Update the provider API to actually return diagnostics here,
	// and then we can remove all this shimming and use its diagnostics
	// directly.
	for _, warn := range warns {
		diags = diags.Append(tfdiags.SimpleWarning(warn))
	}
	for _, err := range errs {
		diags = diags.Append(err)
	}

	if n.IgnoreWarnings {
		// If we _only_ have warnings then we'll return nil.
		if diags.HasErrors() {
			return nil, diags.NonFatalErr()
		}
		return nil, nil
	} else {
		// We'll return an error if there are any diagnostics at all, even if
		// some of them are warnings.
		return nil, diags.NonFatalErr()
	}
}
