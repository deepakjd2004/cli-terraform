package cloudlets

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"
	"text/template"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v3/pkg/cloudlets"
	"github.com/akamai/cli-terraform/pkg/templates"
	"github.com/akamai/cli-terraform/pkg/tools"
	"github.com/akamai/cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockProcessor struct {
	mock.Mock
}

func (m *mockProcessor) ProcessTemplates(i interface{}) error {
	args := m.Called(i)
	return args.Error(0)
}

func TestMain(m *testing.M) {
	if err := os.MkdirAll("./testdata/res", 0755); err != nil {
		log.Fatal(err)
	}
	exitCode := m.Run()
	if err := os.RemoveAll("./testdata/res"); err != nil {
		log.Fatal(err)
	}
	os.Exit(exitCode)
}

func TestCreatePolicy(t *testing.T) {
	section := "test_section"
	pageSize := 1000
	tests := map[string]struct {
		init      func(*cloudlets.Mock, *mockProcessor)
		withError error
	}{
		"fetch latest version of policy and produce output ALB": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ALB",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ALB",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleALB{
							Name:  "some rule",
							Type:  "ALB",
							Start: 1,
							End:   2,
							ID:    1234,
							ForwardSettings: cloudlets.ForwardSettingsALB{
								OriginID: "test_origin",
							},
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()

				origin := cloudlets.Origin{
					OriginID:    "test_origin",
					Description: "test description",
					Type:        "APPLICATION_LOAD_BALANCER",
				}

				var versionList []cloudlets.LoadBalancerVersion
				for i := 1; i <= 2; i++ {
					versionList = append(versionList, cloudlets.LoadBalancerVersion{OriginID: origin.OriginID, Version: int64(i)})
				}
				c.On("ListLoadBalancerVersions", mock.Anything, cloudlets.ListLoadBalancerVersionsRequest{
					OriginID: origin.OriginID,
				}).Return(versionList, nil).Once()

				activations := []cloudlets.LoadBalancerActivation{
					{
						ActivatedDate: "2021-10-29T00:00:10.000Z",
						Network:       cloudlets.LoadBalancerActivationNetworkProduction,
						OriginID:      origin.OriginID,
						Status:        cloudlets.LoadBalancerActivationStatusActive,
						Version:       2,
					},
					{
						ActivatedDate: "2021-10-29T00:00:20.000Z",
						Network:       cloudlets.LoadBalancerActivationNetworkStaging,
						OriginID:      origin.OriginID,
						Status:        cloudlets.LoadBalancerActivationStatusActive,
						Version:       2,
					},
				}
				c.On("ListLoadBalancerActivations", mock.Anything, cloudlets.ListLoadBalancerActivationsRequest{
					OriginID: origin.OriginID,
				}).Return(activations, nil).Twice()

				p.On("ProcessTemplates", TFPolicyData{
					Name:              "test_policy",
					Section:           section,
					CloudletCode:      "ALB",
					Description:       "version 2 description",
					GroupID:           234,
					PolicyActivations: map[string]TFPolicyActivationData{},
					MatchRuleFormat:   "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleALB{
							Name:  "some rule",
							Type:  "ALB",
							Start: 1,
							End:   2,
							ID:    1234,
							ForwardSettings: cloudlets.ForwardSettingsALB{
								OriginID: "test_origin",
							},
						},
					},
					LoadBalancers:           versionList[1:],
					LoadBalancerActivations: activations,
				}).Return(nil).Once()
			},
		},
		"fetch latest version of policy and produce output with activations ER": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ER",
						Activations: []cloudlets.PolicyActivation{
							{
								Network: "staging",
								PolicyInfo: cloudlets.PolicyInfo{
									Version: 2,
								},
								PropertyInfo: cloudlets.PropertyInfo{
									Name: "test_prp_1",
								},
							},
							{
								Network: "prod",
								PolicyInfo: cloudlets.PolicyInfo{
									Version: 1,
								},
								PropertyInfo: cloudlets.PropertyInfo{
									Name: "test_prp_1",
								},
							},
							{
								Network: "staging",
								PolicyInfo: cloudlets.PolicyInfo{
									Version: 2,
								},
								PropertyInfo: cloudlets.PropertyInfo{
									Name: "test_prp_2",
								},
							},
						},
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleER{
							Name:  "some rule",
							Type:  "ER",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()
				p.On("ProcessTemplates", TFPolicyData{
					Name:            "test_policy",
					Section:         section,
					CloudletCode:    "ER",
					Description:     "version 2 description",
					GroupID:         234,
					MatchRuleFormat: "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleER{
							Name:  "some rule",
							Type:  "ER",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
					PolicyActivations: map[string]TFPolicyActivationData{
						"staging": {
							PolicyID:   2,
							Version:    2,
							Properties: []string{"test_prp_1", "test_prp_2"},
						},
						"prod": {
							PolicyID:   2,
							Version:    1,
							Properties: []string{"test_prp_1"},
						},
					},
				}).Return(nil).Once()
			},
		},
		"fetch latest version of policy and produce output with activations CD": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "CD",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "CD",
						Activations: []cloudlets.PolicyActivation{
							{
								Network: "staging",
								PolicyInfo: cloudlets.PolicyInfo{
									Version: 2,
								},
								PropertyInfo: cloudlets.PropertyInfo{
									Name: "test_prp_1",
								},
							},
							{
								Network: "prod",
								PolicyInfo: cloudlets.PolicyInfo{
									Version: 1,
								},
								PropertyInfo: cloudlets.PropertyInfo{
									Name: "test_prp_1",
								},
							},
							{
								Network: "staging",
								PolicyInfo: cloudlets.PolicyInfo{
									Version: 2,
								},
								PropertyInfo: cloudlets.PropertyInfo{
									Name: "test_prp_2",
								},
							},
						},
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRulePR{
							Name:  "some rule",
							Type:  "CD",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()
				p.On("ProcessTemplates", TFPolicyData{
					Name:            "test_policy",
					Section:         section,
					CloudletCode:    "CD",
					Description:     "version 2 description",
					GroupID:         234,
					MatchRuleFormat: "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRulePR{
							Name:  "some rule",
							Type:  "CD",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
					PolicyActivations: map[string]TFPolicyActivationData{
						"staging": {
							PolicyID:   2,
							Version:    2,
							Properties: []string{"test_prp_1", "test_prp_2"},
						},
						"prod": {
							PolicyID:   2,
							Version:    1,
							Properties: []string{"test_prp_1"},
						},
					},
				}).Return(nil).Once()
			},
		},
		"fetch latest version of policy and produce output without activations": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ER",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleER{
							Name:  "some rule",
							Type:  "ER",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()
				p.On("ProcessTemplates", TFPolicyData{
					Name:              "test_policy",
					Section:           section,
					CloudletCode:      "ER",
					Description:       "version 2 description",
					GroupID:           234,
					PolicyActivations: map[string]TFPolicyActivationData{},
					MatchRuleFormat:   "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleER{
							Name:  "some rule",
							Type:  "ER",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
				}).Return(nil).Once()
			},
		},
		"fetch latest version of policy and produce output without activations AP": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "AP",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "AP",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleAP{
							Name:               "some rule",
							Type:               "AP",
							Start:              1,
							End:                2,
							ID:                 1234,
							PassThroughPercent: tools.Float64Ptr(100),
							Disabled:           true,
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()
				p.On("ProcessTemplates", TFPolicyData{
					Name:              "test_policy",
					Section:           section,
					CloudletCode:      "AP",
					Description:       "version 2 description",
					GroupID:           234,
					PolicyActivations: map[string]TFPolicyActivationData{},
					MatchRuleFormat:   "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleAP{
							Name:               "some rule",
							Type:               "AP",
							Start:              1,
							End:                2,
							ID:                 1234,
							PassThroughPercent: tools.Float64Ptr(100),
							Disabled:           true,
						},
					},
				}).Return(nil).Once()
			},
		},
		"fetch latest version of policy and produce output without activations AS": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      11,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "AS",
					},
					{
						PolicyID:     2,
						GroupID:      22,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "AS",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleAS{
							Name:     "a rule",
							Type:     "AS",
							Start:    1,
							End:      2,
							ID:       1000,
							Disabled: true,
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()
				p.On("ProcessTemplates", TFPolicyData{
					Name:              "test_policy",
					Section:           section,
					CloudletCode:      "AS",
					Description:       "version 2 description",
					GroupID:           22,
					PolicyActivations: map[string]TFPolicyActivationData{},
					MatchRuleFormat:   "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleAS{
							Name:     "a rule",
							Type:     "AS",
							Start:    1,
							End:      2,
							ID:       1000,
							Disabled: true,
						},
					},
				}).Return(nil).Once()
			},
		},
		"error fetching policy": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return(nil, fmt.Errorf("oops")).Once()
			},
			withError: ErrFetchingPolicy,
		},
		"error policy not found": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
				}, nil).Once()
			},
			withError: ErrFetchingPolicy,
		},
		"unsupported cloudlet type": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ABC",
					},
				}, nil).Once()
			},
			withError: ErrCloudletTypeNotSupported,
		},
		"error listing versions": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ER",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return(nil, fmt.Errorf("oops")).Once()
			},
			withError: ErrFetchingVersion,
		},
		"error fetching latest version": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ER",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(nil, fmt.Errorf("oops")).Once()
			},
			withError: ErrFetchingVersion,
		},
		"error processing template": {
			init: func(c *cloudlets.Mock, p *mockProcessor) {
				c.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{
						PolicyID:     1,
						GroupID:      123,
						Name:         "some policy",
						CloudletID:   0,
						CloudletCode: "ER",
					},
					{
						PolicyID:     2,
						GroupID:      234,
						Name:         "test_policy",
						Description:  "test_policy description",
						CloudletID:   0,
						CloudletCode: "ER",
					},
				}, nil).Once()
				c.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 2, PageSize: &pageSize, Offset: 0}).Return([]cloudlets.PolicyVersion{
					{
						PolicyID: 2,
						Version:  1,
					},
					{
						PolicyID:        2,
						Version:         2,
						Description:     "version 2 description",
						MatchRuleFormat: "1.0",
					},
				}, nil).Once()
				c.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{
					PolicyID: 2,
					Version:  2,
				}).Return(&cloudlets.PolicyVersion{
					PolicyID:    2,
					Version:     2,
					Description: "version 2 description",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleER{
							Name:  "some rule",
							Type:  "ER",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
					MatchRuleFormat: "1.0",
				}, nil).Once()
				p.On("ProcessTemplates", TFPolicyData{
					Name:              "test_policy",
					Section:           section,
					CloudletCode:      "ER",
					Description:       "version 2 description",
					GroupID:           234,
					PolicyActivations: map[string]TFPolicyActivationData{},
					MatchRuleFormat:   "1.0",
					MatchRules: cloudlets.MatchRules{
						&cloudlets.MatchRuleER{
							Name:  "some rule",
							Type:  "ER",
							Start: 1,
							End:   2,
							ID:    1234,
						},
					},
				}).Return(templates.ErrSavingFiles).Once()
			},
			withError: templates.ErrSavingFiles,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mc := new(cloudlets.Mock)
			mp := new(mockProcessor)
			test.init(mc, mp)
			ctx := terminal.Context(context.Background(), terminal.New(terminal.DiscardWriter(), nil, terminal.DiscardWriter()))
			err := createPolicy(ctx, "test_policy", section, mc, mp)
			if test.withError != nil {
				assert.True(t, errors.Is(err, test.withError), "expected: %s; got: %s", test.withError, err)
				return
			}
			require.NoError(t, err)
			mc.AssertExpectations(t)
			mp.AssertExpectations(t)
		})
	}
}

func TestProcessPolicyTemplates(t *testing.T) {
	tests := map[string]struct {
		givenData    TFPolicyData
		dir          string
		filesToCheck []string
	}{
		"policy with ER match rules and activations": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ER",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				PolicyActivations: map[string]TFPolicyActivationData{
					"staging": {
						PolicyID:   2,
						Version:    2,
						Properties: []string{"prp_0", "prp_1"},
					},
					"prod": {
						PolicyID:   2,
						Version:    1,
						Properties: []string{"prp_0"},
					},
				},
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleER{
						Name:  "r1",
						Start: 1,
						End:   2,
						Matches: []cloudlets.MatchCriteriaER{
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "extension",
								MatchValue:    "txt",
								MatchOperator: "equals",
							},
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               307,
						RedirectURL:              "/abc/sss",
						MatchURL:                 "test.url",
						UseIncomingSchemeAndHost: true,
					},
					cloudlets.MatchRuleER{
						Name:                     "r2",
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               301,
						RedirectURL:              "/ddd",
						MatchURL:                 "abc.com",
						UseIncomingSchemeAndHost: true,
						Matches: []cloudlets.MatchCriteriaER{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "ALB",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
					},
				},
			},
			dir:          "with_activations_and_match_rules",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy with ER match rules and single activation": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ER",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				PolicyActivations: map[string]TFPolicyActivationData{
					"prod": {
						PolicyID:   2,
						Version:    1,
						Properties: []string{"prp_0"},
					},
				},
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleER{
						Name:  "r1",
						Start: 1,
						End:   2,
						Matches: []cloudlets.MatchCriteriaER{
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "extension",
								MatchValue:    "txt",
								MatchOperator: "equals",
							},
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               307,
						RedirectURL:              "/abc/sss",
						MatchURL:                 "test.url",
						UseIncomingSchemeAndHost: true,
					},
					cloudlets.MatchRuleER{
						Name:                     "r2",
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               301,
						RedirectURL:              "/ddd",
						MatchURL:                 "abc.com",
						UseIncomingSchemeAndHost: true,
						Matches: []cloudlets.MatchCriteriaER{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "ALB",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
					},
				},
			},
			dir:          "with_single_activation",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ER",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleER{
						Name:  "r1",
						Start: 1,
						End:   2,
						Matches: []cloudlets.MatchCriteriaER{
							{
								MatchType:     "extension",
								MatchValue:    "txt",
								MatchOperator: "equals",
							},
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               307,
						RedirectURL:              "/abc/sss",
						MatchURL:                 "test.url",
						UseIncomingSchemeAndHost: true,
					},
					cloudlets.MatchRuleER{
						Name:                     "r2",
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               301,
						RedirectURL:              "/ddd",
						MatchURL:                 "abc.com",
						UseIncomingSchemeAndHost: true,
					},
				},
			},
			dir:          "no_activations_with_match_rules",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules and invalid escape er": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ER",
				Description:     `Testing\ exported policy`,
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleER{
						Name:                     `\r2`,
						UseRelativeURL:           "copy_scheme_hostname",
						StatusCode:               301,
						RedirectURL:              `/\ddd`,
						MatchURL:                 `abc.\com`,
						UseIncomingSchemeAndHost: true,
						Matches: []cloudlets.MatchCriteriaER{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								MatchValue:    `value\`,
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: `ER\`,
									Options: &cloudlets.Options{
										Value:            []string{`\y`},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
					},
				},
			},
			dir:          "no_activations_with_escaped_strings_er",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules and invalid escape alb": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ALB",
				Description:     `Testing\ exported policy`,
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleALB{
						Name: `\r2`,
						Matches: []cloudlets.MatchCriteriaALB{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								MatchValue:    `value\`,
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: `ALB\`,
									Options: &cloudlets.Options{
										Value:            []string{`\y`},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						MatchURL:        `abc.\com`,
						MatchesAlways:   false,
						ForwardSettings: cloudlets.ForwardSettingsALB{},
						Disabled:        false,
					},
				},
				LoadBalancers: []cloudlets.LoadBalancerVersion{
					{
						OriginID:      "test_origin",
						Description:   `test\ description`,
						BalancingType: cloudlets.BalancingTypeWeighted,
						DataCenters: []cloudlets.DataCenter{
							{
								City:            "Boston",
								CloudService:    true,
								Continent:       "NA",
								Country:         "US",
								Hostname:        "test-hostname",
								Latitude:        tools.Float64Ptr(102.78108),
								LivenessHosts:   []string{"tf1.test", "tf2.test"},
								Longitude:       tools.Float64Ptr(-116.07064),
								OriginID:        "test_origin",
								Percent:         tools.Float64Ptr(10),
								StateOrProvince: tools.StringPtr("MA"),
							},
						},
						LivenessSettings: &cloudlets.LivenessSettings{
							HostHeader:        "header",
							AdditionalHeaders: map[string]string{"abc": "123"},
							Interval:          10,
							Path:              `/\status`,
							Port:              1234,
							Protocol:          "HTTP",
							RequestString:     `test_\request_string`,
							ResponseString:    `test_\response_string`,
							Timeout:           60,
						},
						Version: 2,
					},
				},
			},
			dir:          "no_activations_with_escaped_strings_alb",
			filesToCheck: []string{"policy.tf", "load-balancer.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules and two alb": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ALB",
				Description:     `Testing exported policy`,
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleALB{
						Name: `r2`,
						Matches: []cloudlets.MatchCriteriaALB{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								MatchValue:    `value`,
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: `ALB`,
									Options: &cloudlets.Options{
										Value:            []string{`y`},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						MatchURL:        `abc.com`,
						MatchesAlways:   false,
						ForwardSettings: cloudlets.ForwardSettingsALB{},
						Disabled:        false,
					},
				},
				LoadBalancers: []cloudlets.LoadBalancerVersion{
					{
						OriginID:      "test_origin",
						Description:   `test description`,
						BalancingType: cloudlets.BalancingTypeWeighted,
						DataCenters: []cloudlets.DataCenter{
							{
								City:            "Boston",
								CloudService:    true,
								Continent:       "NA",
								Country:         "US",
								Hostname:        "test-hostname",
								Latitude:        tools.Float64Ptr(102.78108),
								LivenessHosts:   []string{"tf1.test", "tf2.test"},
								Longitude:       tools.Float64Ptr(-116.07064),
								OriginID:        "test_origin",
								Percent:         tools.Float64Ptr(10),
								StateOrProvince: tools.StringPtr("MA"),
							},
						},
						LivenessSettings: &cloudlets.LivenessSettings{
							HostHeader:        "header",
							AdditionalHeaders: map[string]string{"abc": "123"},
							Interval:          10,
							Path:              `status`,
							Port:              1234,
							Protocol:          "HTTP",
							RequestString:     `test_request_string`,
							ResponseString:    `test_response_string`,
							Timeout:           60,
						},
						Version: 2,
					},
					{
						OriginID:      "test_origin_2",
						Description:   `test description`,
						BalancingType: cloudlets.BalancingTypeWeighted,
						DataCenters: []cloudlets.DataCenter{
							{
								City:            "Boston",
								CloudService:    true,
								Continent:       "NA",
								Country:         "US",
								Hostname:        "test-hostname",
								Latitude:        tools.Float64Ptr(102.78108),
								LivenessHosts:   []string{"tf1.test", "tf2.test"},
								Longitude:       tools.Float64Ptr(-116.07064),
								OriginID:        "test_origin",
								Percent:         tools.Float64Ptr(10),
								StateOrProvince: tools.StringPtr("MA"),
							},
						},
						LivenessSettings: &cloudlets.LivenessSettings{
							HostHeader:        "header",
							AdditionalHeaders: map[string]string{"abc": "123"},
							Interval:          10,
							Path:              `status`,
							Port:              1234,
							Protocol:          "HTTP",
							RequestString:     `test_request_string`,
							ResponseString:    `test_response_string`,
							Timeout:           60,
						},
						Version: 2,
					},
				},
			},
			dir:          "no_activations_with_two_alb",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "load-balancer.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ER",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_activations_no_match_rules",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules alb": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ALB",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleALB{
						Name: "r1",
						Matches: []cloudlets.MatchCriteriaALB{
							{
								CaseSensitive: false,
								MatchOperator: "equals",
								MatchType:     "range",
								Negate:        false,
								ObjectMatchValue: &cloudlets.ObjectMatchValueRange{
									Type:  "range",
									Value: []int64{1, 50},
								},
							},
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						MatchURL:      "test.url",
						MatchesAlways: false,
						ForwardSettings: cloudlets.ForwardSettingsALB{
							OriginID: "test_origin",
						},
					},
					cloudlets.MatchRuleALB{
						Name:     "r2",
						MatchURL: "abc.com",
						ForwardSettings: cloudlets.ForwardSettingsALB{
							OriginID: "test_origin",
						},
						Matches: []cloudlets.MatchCriteriaALB{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "ALB",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						Disabled: true,
					},
				},
				LoadBalancers: []cloudlets.LoadBalancerVersion{
					{
						OriginID:      "test_origin",
						Description:   "test description",
						BalancingType: cloudlets.BalancingTypeWeighted,
						DataCenters: []cloudlets.DataCenter{
							{
								City:            "Boston",
								CloudService:    true,
								Continent:       "NA",
								Country:         "US",
								Hostname:        "test-hostname",
								Latitude:        tools.Float64Ptr(102.78108),
								LivenessHosts:   []string{"tf1.test", "tf2.test"},
								Longitude:       tools.Float64Ptr(-116.07064),
								OriginID:        "test_origin",
								Percent:         tools.Float64Ptr(10),
								StateOrProvince: tools.StringPtr("MA"),
							},
						},
						LivenessSettings: &cloudlets.LivenessSettings{
							HostHeader:        "header",
							AdditionalHeaders: map[string]string{"abc": "123"},
							Interval:          10,
							Path:              "/status",
							Port:              1234,
							Protocol:          "HTTP",
							RequestString:     "test_request_string",
							ResponseString:    "test_response_string",
							Timeout:           60,
						},
						Version: 2,
					},
				},
			},
			dir:          "with_match_rules_alb",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "load-balancer.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules alb and activations": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ALB",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleALB{
						Name: "r1",
						Matches: []cloudlets.MatchCriteriaALB{
							{
								CaseSensitive: false,
								MatchOperator: "equals",
								MatchType:     "range",
								Negate:        false,
								ObjectMatchValue: &cloudlets.ObjectMatchValueRange{
									Type:  "range",
									Value: []int64{1, 50},
								},
							},
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						MatchURL:      "test.url",
						MatchesAlways: false,
						ForwardSettings: cloudlets.ForwardSettingsALB{
							OriginID: "test_origin",
						},
						Disabled: false,
					},
					cloudlets.MatchRuleALB{
						Name:     "r2",
						MatchURL: "abc.com",
						ForwardSettings: cloudlets.ForwardSettingsALB{
							OriginID: "test_origin",
						},
						Matches: []cloudlets.MatchCriteriaALB{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "ALB",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						Disabled: true,
					},
				},
				LoadBalancers: []cloudlets.LoadBalancerVersion{
					{
						OriginID:      "test_origin",
						Description:   "test description",
						BalancingType: cloudlets.BalancingTypeWeighted,
						DataCenters: []cloudlets.DataCenter{
							{
								City:            "Boston",
								CloudService:    true,
								Continent:       "NA",
								Country:         "US",
								Hostname:        "test-hostname",
								Latitude:        tools.Float64Ptr(102.78108),
								LivenessHosts:   []string{"tf1.test", "tf2.test"},
								Longitude:       tools.Float64Ptr(-116.07064),
								OriginID:        "test_origin",
								Percent:         tools.Float64Ptr(10),
								StateOrProvince: tools.StringPtr("MA"),
							},
						},
						LivenessSettings: &cloudlets.LivenessSettings{
							HostHeader:        "header",
							AdditionalHeaders: map[string]string{"abc": "123"},
							Interval:          10,
							Path:              "/status",
							Port:              1234,
							Protocol:          "HTTP",
							RequestString:     "test_request_string",
							ResponseString:    "test_response_string",
							Timeout:           60,
						},
						Version: 2,
					},
				},
				LoadBalancerActivations: []cloudlets.LoadBalancerActivation{
					{
						ActivatedDate: "2021-10-29T00:00:10.000Z",
						Network:       cloudlets.LoadBalancerActivationNetworkProduction,
						OriginID:      "test_origin",
						Status:        cloudlets.LoadBalancerActivationStatusActive,
						Version:       2,
					},
					{
						ActivatedDate: "2021-10-29T00:00:20.000Z",
						Network:       cloudlets.LoadBalancerActivationNetworkStaging,
						OriginID:      "test_origin",
						Status:        cloudlets.LoadBalancerActivationStatusActive,
						Version:       2,
					},
				},
			},
			dir:          "with_activations_and_match_rules_alb",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "load-balancer.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules alb": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "ALB",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_alb",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules fr": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "FR",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_fr",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules CD": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "CD",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_cd",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules fr": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "FR",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleFR{
						Name: "r1",
						Matches: []cloudlets.MatchCriteriaFR{
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						MatchURL: "test.url",
						ForwardSettings: cloudlets.ForwardSettingsFR{
							PathAndQS:              "/test",
							UseIncomingQueryString: false,
							OriginID:               "test_origin",
						},
						Disabled: false,
					},
					cloudlets.MatchRuleFR{
						Name:     "r2",
						MatchURL: "abc.com",
						ForwardSettings: cloudlets.ForwardSettingsFR{
							OriginID: "test_origin",
						},
						Matches: []cloudlets.MatchCriteriaFR{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "test_omv",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						Disabled: true,
					},
				},
			},
			dir:          "with_match_rules_fr",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules CD": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "CD",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRulePR{
						Name: "r1",
						Matches: []cloudlets.MatchCriteriaPR{
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						MatchURL: "test.url",
						ForwardSettings: cloudlets.ForwardSettingsPR{
							OriginID: "test_origin",
							Percent:  1,
						},
						Disabled: false,
					},
					cloudlets.MatchRulePR{
						Name:     "r2",
						MatchURL: "abc.com",
						ForwardSettings: cloudlets.ForwardSettingsPR{
							OriginID: "test_origin",
							Percent:  1,
						},
						Matches: []cloudlets.MatchCriteriaPR{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "test_omv",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						Disabled:      true,
						MatchesAlways: true,
					},
				},
			},
			dir:          "with_match_rules_cd",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules vp": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "VP",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleVP{
						Name: "r1",
						Matches: []cloudlets.MatchCriteriaVP{
							{
								MatchType:     "cookie",
								MatchValue:    "cookie=cookievalue",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						MatchURL:           "test.url",
						Disabled:           false,
						PassThroughPercent: tools.Float64Ptr(100),
					},
					cloudlets.MatchRuleVP{
						Name:     "r2",
						MatchURL: "abc.com",
						Matches: []cloudlets.MatchCriteriaVP{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "VP",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						PassThroughPercent: tools.Float64Ptr(-1),
					},
					cloudlets.MatchRuleVP{
						Name:               "r3",
						PassThroughPercent: tools.Float64Ptr(50.55),
						Disabled:           true,
					},
				},
			},
			dir:          "with_match_rules_vp",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules vp": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "VP",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_vp",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules ap": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "AP",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleAP{
						Name: "r1",
						Matches: []cloudlets.MatchCriteriaAP{
							{
								MatchType:     "method",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
							{
								MatchType:     "hostname",
								MatchValue:    "3333.dom",
								MatchOperator: "equals",
								CaseSensitive: true,
								Negate:        true,
							},
						},
						MatchURL:           "test.url",
						Disabled:           false,
						PassThroughPercent: tools.Float64Ptr(100),
					},
					cloudlets.MatchRuleAP{
						Name:     "r2",
						MatchURL: "abc.com",
						Matches: []cloudlets.MatchCriteriaAP{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "AP",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						PassThroughPercent: tools.Float64Ptr(-1),
					},
					cloudlets.MatchRuleAP{
						Name:               "r3",
						PassThroughPercent: tools.Float64Ptr(50.55),
						Disabled:           true,
					},
				},
			},
			dir:          "with_match_rules_ap",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules ap": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "AP",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_ap",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules as": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "AS",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_as",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules as": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "AS",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleAS{
						Name: "rule1",
						Matches: []cloudlets.MatchCriteriaAS{
							{
								MatchType:     "method",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
						},
						ForwardSettings: cloudlets.ForwardSettingsAS{
							PathAndQS: "some_path",
						},
						MatchURL: "test.url",
						Disabled: false,
					},
					cloudlets.MatchRuleAS{
						Name:     "rule2",
						MatchURL: "abc.com",
						Matches: []cloudlets.MatchCriteriaAS{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "AS",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						ForwardSettings: cloudlets.ForwardSettingsAS{
							UseIncomingQueryString: true,
						},
					},
					cloudlets.MatchRuleAS{
						Name:  "rule3",
						Start: 1,
						End:   2,
						Matches: []cloudlets.MatchCriteriaAS{
							{
								MatchType:     "range",
								MatchOperator: "equals",
								ObjectMatchValue: &cloudlets.ObjectMatchValueRange{
									Type:  "range",
									Value: []int64{1, 50},
								},
							},
						},
						MatchURL: "test.url",
						ForwardSettings: cloudlets.ForwardSettingsAS{
							OriginID: "test_origin",
						},
						Disabled: false,
					},
					cloudlets.MatchRuleAS{
						Name:     "rule_empty",
						Disabled: true,
					},
				},
			},
			dir:          "with_match_rules_as",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
		"policy without match rules ig": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "IG",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
			},
			dir:          "no_match_rules_ig",
			filesToCheck: []string{"policy.tf", "variables.tf", "import.sh"},
		},
		"policy with match rules ig": {
			givenData: TFPolicyData{
				Name:            "test_policy_export",
				Section:         "test_section",
				CloudletCode:    "IG",
				Description:     "Testing exported policy",
				GroupID:         12345,
				MatchRuleFormat: "1.0",
				MatchRules: cloudlets.MatchRules{
					cloudlets.MatchRuleRC{
						Name: "rule1",
						Matches: []cloudlets.MatchCriteriaRC{
							{
								MatchType:     "method",
								MatchOperator: "equals",
								CaseSensitive: true,
								ObjectMatchValue: cloudlets.ObjectMatchValueSimple{
									Type:  "simple",
									Value: []string{"GET"},
								},
							},
						},
						AllowDeny: cloudlets.Allow,
						Disabled:  false,
					},
					cloudlets.MatchRuleRC{
						Name: "rule2",
						Matches: []cloudlets.MatchCriteriaRC{
							{
								MatchOperator: "equals",
								MatchType:     "header",
								ObjectMatchValue: cloudlets.ObjectMatchValueObject{
									Type: "object",
									Name: "Accept",
									Options: &cloudlets.Options{
										Value:            []string{"y"},
										ValueHasWildcard: true,
									},
								},
								Negate: false,
							},
						},
						AllowDeny: cloudlets.Allow,
					},
					cloudlets.MatchRuleRC{
						Name:          "rule_empty",
						AllowDeny:     cloudlets.Deny,
						MatchesAlways: true,
						Disabled:      true,
					},
				},
			},
			dir:          "with_match_rules_ig",
			filesToCheck: []string{"policy.tf", "match-rules.tf", "variables.tf", "import.sh"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.MkdirAll(fmt.Sprintf("./testdata/res/%s", test.dir), 0755))
			processor := templates.FSTemplateProcessor{
				TemplatesFS: templateFiles,
				TemplateTargets: map[string]string{
					"policy.tmpl":        fmt.Sprintf("./testdata/res/%s/policy.tf", test.dir),
					"match-rules.tmpl":   fmt.Sprintf("./testdata/res/%s/match-rules.tf", test.dir),
					"load-balancer.tmpl": fmt.Sprintf("./testdata/res/%s/load-balancer.tf", test.dir),
					"variables.tmpl":     fmt.Sprintf("./testdata/res/%s/variables.tf", test.dir),
					"imports.tmpl":       fmt.Sprintf("./testdata/res/%s/import.sh", test.dir),
				},
				AdditionalFuncs: template.FuncMap{
					"deepequal": reflect.DeepEqual,
				},
			}
			require.NoError(t, processor.ProcessTemplates(test.givenData))

			for _, f := range test.filesToCheck {
				expected, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/%s", test.dir, f))
				require.NoError(t, err)
				result, err := ioutil.ReadFile(fmt.Sprintf("./testdata/res/%s/%s", test.dir, f))
				require.NoError(t, err)
				assert.Equal(t, string(expected), string(result))
			}
		})
	}
}

func TestFindPolicy(t *testing.T) {
	pageSize := 1000
	preparePoliciesPage := func(pageSize, startingID int64) []cloudlets.Policy {
		policies := make([]cloudlets.Policy, 0, pageSize)
		for i := startingID; i < startingID+pageSize; i++ {
			policies = append(policies, cloudlets.Policy{PolicyID: i, Name: fmt.Sprintf("%d", i)})
		}
		return policies
	}
	tests := map[string]struct {
		policyName string
		init       func(m *cloudlets.Mock)
		expectedID int64
		withError  bool
	}{
		"policy found in first iteration": {
			policyName: "test_policy",
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).Return([]cloudlets.Policy{
					{PolicyID: 9999999, Name: "some_policy"},
					{PolicyID: 1234567, Name: "test_policy"},
				}, nil).Once()
			},
			expectedID: 1234567,
		},
		"policy found on 3rd page": {
			policyName: "test_policy",
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).
					Return(preparePoliciesPage(1000, 0), nil).Once()
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 1000}).
					Return(preparePoliciesPage(1000, 1000), nil).Once()
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 2000}).Return([]cloudlets.Policy{
					{PolicyID: 9999999, Name: "some_policy"},
					{PolicyID: 1234567, Name: "test_policy"},
				}, nil).Once()

			},
			expectedID: 1234567,
		},
		"policy not found": {
			policyName: "test_policy",
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).
					Return(preparePoliciesPage(1000, 0), nil).Once()
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 1000}).
					Return(preparePoliciesPage(1000, 1000), nil).Once()
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 2000}).
					Return(preparePoliciesPage(250, 2000), nil).Once()

			},
			withError: true,
		},
		"error listing policies": {
			policyName: "test_policy",
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 0}).
					Return(preparePoliciesPage(1000, 0), nil).Once()
				m.On("ListPolicies", mock.Anything, cloudlets.ListPoliciesRequest{PageSize: &pageSize, Offset: 1000}).
					Return(nil, fmt.Errorf("oops")).Once()

			},
			withError: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := new(cloudlets.Mock)
			test.init(m)
			policy, err := findPolicyByName(context.Background(), test.policyName, m)
			m.AssertExpectations(t)
			if test.withError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expectedID, policy.PolicyID)
		})
	}
}

func TestGetLatestPolicyVersion(t *testing.T) {
	pageSize := 1000
	prepareVersionsPage := func(pageSize, startingVersion int64) []cloudlets.PolicyVersion {
		versions := make([]cloudlets.PolicyVersion, 0, pageSize)
		for i := startingVersion; i < startingVersion+pageSize; i++ {
			versions = append(versions, cloudlets.PolicyVersion{Version: i})
		}
		return versions
	}
	tests := map[string]struct {
		policyID  int64
		init      func(m *cloudlets.Mock)
		expected  int64
		withError bool
	}{
		"policy version found in first iteration": {
			policyID: 123,
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 0}).
					Return(prepareVersionsPage(500, 0), nil).Once()
				m.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{PolicyID: 123, Version: 499}).
					Return(&cloudlets.PolicyVersion{Version: 499}, nil).Once()
			},
			expected: 499,
		},
		"policy version found on 3rd page": {
			policyID: 123,
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 0}).
					Return(prepareVersionsPage(1000, 0), nil).Once()
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 1000}).
					Return(prepareVersionsPage(1000, 1000), nil).Once()
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 2000}).
					Return(prepareVersionsPage(500, 2000), nil).Once()
				m.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{PolicyID: 123, Version: 2499}).
					Return(&cloudlets.PolicyVersion{Version: 2499}, nil).Once()
			},
			expected: 2499,
		},
		"no policy versions found": {
			policyID: 123,
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 0}).
					Return([]cloudlets.PolicyVersion{}, nil).Once()
			},
			withError: true,
		},
		"error listing policy versions": {
			policyID: 123,
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 0}).
					Return(nil, fmt.Errorf("oops")).Once()
			},
			withError: true,
		},
		"error fetching latest policy version": {
			policyID: 123,
			init: func(m *cloudlets.Mock) {
				m.On("ListPolicyVersions", mock.Anything, cloudlets.ListPolicyVersionsRequest{PolicyID: 123, PageSize: &pageSize, Offset: 0}).
					Return(prepareVersionsPage(500, 0), nil).Once()
				m.On("GetPolicyVersion", mock.Anything, cloudlets.GetPolicyVersionRequest{PolicyID: 123, Version: 499}).
					Return(nil, fmt.Errorf("oops")).Once()
			},
			withError: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := new(cloudlets.Mock)
			test.init(m)
			policyVersion, err := getLatestPolicyVersion(context.Background(), test.policyID, m)
			m.AssertExpectations(t)
			if test.withError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, policyVersion.Version)
		})
	}
}
