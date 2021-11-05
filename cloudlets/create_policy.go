package cloudlets

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v2/pkg/cloudlets"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v2/pkg/session"
	common "github.com/akamai/cli-common-golang"
	"github.com/akamai/cli-terraform/templates"
	"github.com/akamai/cli-terraform/tools"
	"github.com/fatih/color"
	"github.com/urfave/cli"
)

type (
	// TFPolicyData represents the data used in policy templates
	TFPolicyData struct {
		Name            string
		CloudletCode    string
		Description     string
		GroupID         int64
		MatchRuleFormat cloudlets.MatchRuleFormat
		MatchRules      cloudlets.MatchRules
		Activations     []TFActivationData
	}

	// TFActivationData represents data used in policy activation resource templates
	TFActivationData struct {
		PolicyID   int64
		Network    string
		Version    int64
		Properties []string
	}
)

//go:embed templates/*
var templateFiles embed.FS

var supportedCloudlets = map[string]struct{}{
	"ER": {},
}

var (
	// ErrFetchingPolicy is returned when fetching policy fails
	ErrFetchingPolicy = errors.New("unable to fetch policy with given name")
	// ErrFetchingVersion is returned when fetching policy version fails
	ErrFetchingVersion = errors.New("unable to fetch latest policy version")
	// ErrCloudletTypeNotSupported is returned when a provided cloudlet type is not yet supported
	ErrCloudletTypeNotSupported = errors.New("cloudlet type not supported")
	// ErrSavingFiles is returned when an issue with processing templates occurs
	ErrSavingFiles = errors.New("saving terraform project files")
)

// CmdCreatePolicy is an entrypoint to create-policy command
func CmdCreatePolicy(c *cli.Context) error {
	// TODO context should be retrieved from cli.Context once we migrate to urfave/cli v2
	ctx := context.Background()
	if c.NArg() == 0 {
		if err := cli.ShowCommandHelp(c, c.Command.Name); err != nil {
			return cli.NewExitError(color.RedString("Error displaying help command"), 1)
		}
		return cli.NewExitError(color.RedString("Policy name is required"), 1)
	}
	config, err := tools.GetEdgegridConfig(c)
	if err != nil {
		return err
	}

	sess, err := session.New(
		session.WithSigner(config),
	)
	if err != nil {
		return cli.NewExitError(color.RedString(err.Error()), 1)
	}
	client := cloudlets.Client(sess)
	if c.IsSet("tfworkpath") {
		tools.TFWorkPath = c.String("tfworkpath")
	}
	tools.TFWorkPath = filepath.FromSlash(tools.TFWorkPath)
	if stat, err := os.Stat(tools.TFWorkPath); err != nil || !stat.IsDir() {
		return cli.NewExitError(color.RedString("Destination work path is not accessible"), 1)
	}

	policyPath := filepath.Join(tools.TFWorkPath, "policy.tf")
	variablesPath := filepath.Join(tools.TFWorkPath, "variables.tf")
	importPath := filepath.Join(tools.TFWorkPath, "import.sh")

	err = tools.CheckFiles(policyPath, variablesPath, importPath)
	if err != nil {
		return cli.NewExitError(color.RedString(err.Error()), 1)
	}
	templateToFile := map[string]string{
		"policy.tmpl":    policyPath,
		"variables.tmpl": variablesPath,
		"imports.tmpl":   importPath,
	}

	processor := templates.FSTemplateProcessor{
		TemplatesFS:     templateFiles,
		TemplateTargets: templateToFile,
	}

	policyName := c.Args().First()
	if err = createPolicy(ctx, policyName, client, processor); err != nil {
		return cli.NewExitError(color.RedString(fmt.Sprintf("Error exporting policy HCL: %s", err)), 1)
	}
	return nil
}

func createPolicy(ctx context.Context, policyName string, client cloudlets.Cloudlets, templateProcessor templates.TemplateProcessor) error {
	var tfPolicyData TFPolicyData

	fmt.Println("Configuring Policy")
	common.StartSpinner("Fetching policy "+policyName, "")

	policy, err := findPolicy(ctx, policyName, client)
	if err != nil {
		common.StopSpinnerFail()
		return fmt.Errorf("%w: %s", ErrFetchingPolicy, err)
	}
	if _, ok := supportedCloudlets[policy.CloudletCode]; !ok {
		common.StopSpinnerFail()
		return fmt.Errorf("%w: %s", ErrCloudletTypeNotSupported, policy.CloudletCode)
	}

	tfPolicyData.Name = policy.Name
	tfPolicyData.CloudletCode = policy.CloudletCode
	tfPolicyData.GroupID = policy.GroupID

	policyVersion, err := getLatestPolicyVersion(ctx, policy.PolicyID, client)
	if err != nil {
		common.StopSpinnerFail()
		return fmt.Errorf("%w: %s", ErrFetchingVersion, err)
	}
	tfPolicyData.Description = policyVersion.Description
	tfPolicyData.MatchRuleFormat = policyVersion.MatchRuleFormat
	tfPolicyData.MatchRules = policyVersion.MatchRules

	if activationStaging := getActiveVersionAndProperties(policy, cloudlets.PolicyActivationNetworkStaging); activationStaging != nil {
		tfPolicyData.Activations = append(tfPolicyData.Activations, *activationStaging)
	}
	if activationProd := getActiveVersionAndProperties(policy, cloudlets.PolicyActivationNetworkProduction); activationProd != nil {
		tfPolicyData.Activations = append(tfPolicyData.Activations, *activationProd)
	}

	common.StopSpinnerOk()
	common.StartSpinner("Saving TF configurations ", "")
	if err := templateProcessor.ProcessTemplates(tfPolicyData); err != nil {
		common.StopSpinnerFail()
		return fmt.Errorf("%w: %s", ErrSavingFiles, err)
	}
	common.StopSpinnerOk()
	fmt.Printf("Terraform configuration for policy '%s' was saved successfully\n", policy.Name)

	return nil
}

func findPolicy(ctx context.Context, name string, client cloudlets.Cloudlets) (*cloudlets.Policy, error) {
	pageSize, offset := 1000, 0
	var policy *cloudlets.Policy
	for {
		policies, err := client.ListPolicies(ctx, cloudlets.ListPoliciesRequest{
			Offset:   offset,
			PageSize: &pageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range policies {
			if p.Name == name {
				policy = &p
				return policy, nil
			}
		}
		if len(policies) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil, fmt.Errorf("policy '%s' does not exist", name)
}

func getLatestPolicyVersion(ctx context.Context, policyID int64, client cloudlets.Cloudlets) (*cloudlets.PolicyVersion, error) {
	var version int64
	pageSize, offset := 1000, 0
	for {
		versions, err := client.ListPolicyVersions(ctx, cloudlets.ListPolicyVersionsRequest{
			PolicyID:     policyID,
			IncludeRules: false,
			PageSize:     &pageSize,
			Offset:       offset,
		})
		if err != nil {
			return nil, err
		}

		if len(versions) == 0 {
			return nil, fmt.Errorf("no policy versions found for given policy")
		}
		for _, v := range versions {
			if v.Version > version {
				version = v.Version
			}
		}
		if len(versions) < pageSize {
			break
		}
	}
	policyVersion, err := client.GetPolicyVersion(ctx, cloudlets.GetPolicyVersionRequest{
		PolicyID: policyID,
		Version:  version,
	})
	if err != nil {
		return nil, err
	}
	return policyVersion, nil
}

func getActiveVersionAndProperties(policy *cloudlets.Policy, network cloudlets.PolicyActivationNetwork) *TFActivationData {
	var version int64
	var associatedProperties []string
	for _, activation := range policy.Activations {
		if activation.Network != network {
			continue
		}
		version = activation.PolicyInfo.Version
		associatedProperties = append(associatedProperties, activation.PropertyInfo.Name)
	}
	if associatedProperties == nil {
		return nil
	}
	return &TFActivationData{
		PolicyID:   policy.PolicyID,
		Network:    string(network),
		Version:    version,
		Properties: associatedProperties,
	}
}
