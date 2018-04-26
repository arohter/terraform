package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/terraform/configs"
	"github.com/hashicorp/terraform/terraform"
	"github.com/hashicorp/terraform/tfdiags"
)

// variableValues gathers the variable values provided in default variables
// files, explicit variables files, CLI arguments and environment variables,
// using the given configuration to guide how each variable is processed.
//
// No type checks or conversions are performed here. Instead, values are
// read verbatim and stored for later processing.
//
// This function does not deal with any default values specified for the
// variables in configuration, even though it could. That responsibility is
// left to the functions in the Terraform package to ensure that default
// values will be applied even when the core code is not being entered through
// the CLI entry points.
func (m *Meta) variableValues() (map[string]backend.UnparsedVariableValue, tfdiags.Diagnostics) {
	ret := make(terraform.InputValues)
	var diags tfdiags.Diagnostics

	// Environment variables
	const envVarPrefix = "TF_VAR_"
	for _, varStr := range os.Environ() {
		if !strings.HasPrefix(varStr, envVarPrefix) {
			continue
		}
		eq := strings.Index(varStr, "=")
		if eq == -1 {
			// Weird...
			continue
		}
		name := varStr[len(envVarPrefix):eq]
		rawValue := varStr[eq+1:]

		cv, exists := config[name]
		if !exists {
			// For env vars, unlike all other cases, we tolerate and ignore
			// attempts to set variables that are not declared in the
			// configuration, since that allows a user to leave certain
			// variables set permanently in their shell or by an automation
			// wrapper if they are used across many configurations.
			continue
		}

		val, valDiags := cv.ParsingMode.Parse(name, rawValue)
		diags = diags.Append(valDiags)
		if valDiags.HasErrors() {
			continue
		}
		ret[name] = &terraform.InputValue{
			Value:      val,
			SourceType: terraform.ValueFromEnvVar,
		}
	}

	// We automatically read certain .tfvars and .tfvars.json files if they
	// are present.
	var autoFilenames []string
	if _, err := os.Stat(DefaultVarsFilename); err == nil {
		autoFilenames = append(autoFilenames, DefaultVarsFilename)
	}
	if _, err := os.Stat(DefaultVarsFilename + ".json"); err == nil {
		autoFilenames = append(autoFilenames, DefaultVarsFilename+".json")
	}

	wd, err := os.Getwd()
	if err != nil {
		diags = diags.Append(err)
		return ret, diags
	}
	fis, err := ioutil.ReadDir(wd)
	if err != nil {
		diags = diags.Append(err)
		return ret, diags
	}

	firstNonDefault := len(autoFilenames)

	for _, fi := range fis {
		name := fi.Name()
		// Ignore directories, non-var-files, and ignored files
		if fi.IsDir() || !isAutoVarFile(name) || config.IsIgnoredFile(name) {
			continue
		}
		autoFilenames = append(autoFilenames, name)
	}

	// Sort our filenames, but exclude any DefaultVarsFilename files at the
	// start since these should always be processed first.
	sort.Strings(autoFilenames[firstNonDefault:])

	for _, filename := range autoFilenames {
		fileDiags := m.loadVariableValuesFromFile(filename, config, ret)
		diags = diags.Append(fileDiags)
	}

	// With all of the implicit files out of the way, we'll now deal with
	// the -var and -var-file arguments given on the command line, if any.
	for _, arg := range m.variableArgs {
		switch arg.Name {
		case "-var":
			eq := strings.Index(arg.Value, "=")
			if eq == -1 {
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Invalid -var argument value",
					fmt.Sprintf("The value %q is not in the correct format. -var arguments expect a variable name and a value separated by an equals sign (=).", arg.Value),
				))
				continue
			}
			name := arg.Value[:eq]
			rawValue := arg.Value[eq+1:]
			cv, exists := config[name]
			if !exists {
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Invalid variable name in -var argument",
					fmt.Sprintf("A variable named %q is not declared in the root module. To use this value, declare this variable using a \"variable\" block in the configuration.", name),
				))
				continue
			}

			val, valDiags := cv.ParsingMode.Parse(name, rawValue)
			diags = diags.Append(valDiags)
			if valDiags.HasErrors() {
				continue
			}
			ret[name] = &terraform.InputValue{
				Value:      val,
				SourceType: terraform.ValueFromCLIArg,
			}

		case "-var-file":
			filename := arg.Value
			fileDiags := m.loadVariableValuesFromFile(filename, config, ret)
			diags = diags.Append(fileDiags)

		default:
			diags = diags.Append(tfdiags.SimpleWarning(fmt.Sprintf(
				"Unexpected variable argument name %q. This is a bug in Terraform; please report it in a GitHub issue.",
				arg.Name,
			)))
		}
	}

	return ret, diags
}

func (m *Meta) loadVariableValuesFromFile(filename string, config map[string]*configs.Variable, into terraform.InputValues) tfdiags.Diagnostics {
	var diags tfdiags.Diagnostics

	body, loadDiags := m.loadHCLFile(filename)
	diags = diags.Append(loadDiags)
	if body == nil {
		continue
	}
	attrs, attrDiags := body.JustAttributes()
	diags = diags.Append(attrDiags)

	for name, attr := range attrs {
		vc, exists := config[name]
		if !exists {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Value for undeclared variable",
				Detail:   fmt.Sprintf("The root module does not declare a variable named %q. To use this value, add a \"variable\" block to the configuration.", name),
				Subject:  &attr.NameRange,
			})
			continue
		}

		// We don't enforce types at this layer, so we'll just take whatever
		// value the user provided and check types during context construction.
		val, valDiags := attr.Expr.Value(nil)
		diags = diags.Append(valDiags)
		if valDiags.HasErrors() {
			continue
		}
		into[name] = &terraform.InputValue{
			Value:       val,
			SourceType:  terraform.ValueFromFile,
			SourceRange: attr.Expr.Range(),
		}
	}

	return diags
}
