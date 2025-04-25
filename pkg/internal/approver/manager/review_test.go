/*
Copyright 2021 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manager

import (
	"context"
	"errors"
	"path"
	"testing"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyapi "github.com/cert-manager/approver-policy/pkg/apis/policy/v1alpha1"
	"github.com/cert-manager/approver-policy/pkg/approver"
	"github.com/cert-manager/approver-policy/pkg/approver/fake"
	"github.com/cert-manager/approver-policy/pkg/approver/manager"
	"github.com/cert-manager/approver-policy/pkg/internal/approver/manager/predicate"
	testenv "github.com/cert-manager/approver-policy/test/env"
)

func Test_Review(t *testing.T) {
	env := testenv.RunControlPlane(t, t.Context(),
		testenv.GetenvOrFail(t, "CERT_MANAGER_CRDS"),
		path.Join("..", "..", "..", "..", "deploy", "crds"),
	)

	expNoEvaluation := func(t *testing.T) approver.Evaluator {
		return fake.NewFakeEvaluator().WithEvaluate(func(_ context.Context, _ *policyapi.CertificateRequestPolicy, _ *cmapi.CertificateRequest) (approver.EvaluationResponse, error) {
			t.Fatal("unexpected evaluator call")
			return approver.EvaluationResponse{}, nil
		})
	}

	tests := map[string]struct {
		evaluator   func(t *testing.T) approver.Evaluator
		predicate   func(t *testing.T) predicate.Predicate
		policies    []policyapi.CertificateRequestPolicy
		expResponse manager.ReviewResponse
		expErr      bool
	}{
		"if no CertificateRequestPolicies exist, return ResultUnprocessed": {
			evaluator: expNoEvaluation,
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					t.Fatal("unexpected predicate call")
					return nil, nil
				}
			},
			policies:    nil,
			expResponse: manager.ReviewResponse{Result: manager.ResultUnprocessed, Message: "No CertificateRequestPolicies exist"},
			expErr:      false,
		},
		"if predicate returns an error, return an error": {
			evaluator: expNoEvaluation,
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					return nil, errors.New("this is an error")
				}
			},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
			}},
			expResponse: manager.ReviewResponse{},
			expErr:      true,
		},
		"if predicate returns no policies, return ResultUnprocessed": {
			evaluator: expNoEvaluation,
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					return nil, nil
				}
			},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
			}},
			expResponse: manager.ReviewResponse{Result: manager.ResultUnprocessed, Message: "No CertificateRequestPolicies bound or applicable"},
			expErr:      false,
		},
		"if single policy returns but evaluator denies, return ResultDenied": {
			evaluator: func(t *testing.T) approver.Evaluator {
				return fake.NewFakeEvaluator().WithEvaluate(func(_ context.Context, _ *policyapi.CertificateRequestPolicy, _ *cmapi.CertificateRequest) (approver.EvaluationResponse, error) {
					return approver.EvaluationResponse{Result: approver.ResultDenied, Message: "this is a denied response"}, nil
				})
			},
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					return []policyapi.CertificateRequestPolicy{{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
						Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
					}}, nil
				}
			},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
			}},
			expResponse: manager.ReviewResponse{Result: manager.ResultDenied, Message: "No policy approved this request: [test-policy-a: this is a denied response]"},
			expErr:      false,
		},
		"if single policy returns and evaluator returns not-denied, return ResultApproved": {
			evaluator: func(t *testing.T) approver.Evaluator {
				return fake.NewFakeEvaluator().WithEvaluate(func(_ context.Context, _ *policyapi.CertificateRequestPolicy, _ *cmapi.CertificateRequest) (approver.EvaluationResponse, error) {
					return approver.EvaluationResponse{Result: approver.ResultNotDenied, Message: "this is a not-denied response"}, nil
				})
			},
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					return []policyapi.CertificateRequestPolicy{{
						ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
						Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
					}}, nil
				}
			},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
			}},
			expResponse: manager.ReviewResponse{Result: manager.ResultApproved, Message: `Approved by CertificateRequestPolicy: "test-policy-a"`},
			expErr:      false,
		},
		"if two policies returned and evaluator returns one not-denied, return ResultApproved": {
			evaluator: func(t *testing.T) approver.Evaluator {
				return fake.NewFakeEvaluator().WithEvaluate(func(_ context.Context, policy *policyapi.CertificateRequestPolicy, _ *cmapi.CertificateRequest) (approver.EvaluationResponse, error) {
					if policy.Name == "test-policy-b" {
						return approver.EvaluationResponse{Result: approver.ResultNotDenied, Message: "this is an approved response"}, nil
					}
					return approver.EvaluationResponse{Result: approver.ResultDenied, Message: "this is a denied response"}, nil
				})
			},
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					return []policyapi.CertificateRequestPolicy{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
							Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
							Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
						},
					}, nil
				}
			},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
				},
			},
			expResponse: manager.ReviewResponse{Result: manager.ResultApproved, Message: `Approved by CertificateRequestPolicy: "test-policy-b"`},
			expErr:      false,
		},
		"if two policies returned and both return denied, return ResultDenied": {
			evaluator: func(t *testing.T) approver.Evaluator {
				return fake.NewFakeEvaluator().WithEvaluate(func(_ context.Context, policy *policyapi.CertificateRequestPolicy, _ *cmapi.CertificateRequest) (approver.EvaluationResponse, error) {
					return approver.EvaluationResponse{Result: approver.ResultDenied, Message: "this is a denied response"}, nil
				})
			},
			predicate: func(t *testing.T) predicate.Predicate {
				return func(_ context.Context, _ *cmapi.CertificateRequest, _ []policyapi.CertificateRequestPolicy) ([]policyapi.CertificateRequestPolicy, error) {
					return []policyapi.CertificateRequestPolicy{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
							Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
							Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
						},
					}, nil
				}
			},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec:       policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{}}},
				},
			},
			expResponse: manager.ReviewResponse{Result: manager.ResultDenied, Message: "No policy approved this request: [test-policy-a: this is a denied response] [test-policy-b: this is a denied response]"},
			expErr:      false,
		},
	}

	ctx := t.Context()
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() {
				for _, obj := range test.policies {
					if err := env.AdminClient.Delete(ctx, &obj); /* #nosec G601 -- Func drops pointer at end of call. */ err != nil {
						// Don't Fatal here as a ditch effort to at least try to clean-up
						// everything.
						t.Errorf("failed to delete policy: %s", err)
					}
				}
			})

			for _, obj := range test.policies {
				if err := env.AdminClient.Create(ctx, &obj); /* #nosec G601 -- Func drops pointer at end of call. */ err != nil {
					t.Fatalf("failed to create new policy: %s", err)
				}
			}

			mngr := &mngr{
				lister:     env.AdminClient,
				predicates: []predicate.Predicate{test.predicate(t)},
				evaluators: []approver.Evaluator{test.evaluator(t)},
			}

			response, err := mngr.Review(ctx, &cmapi.CertificateRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "test-req"},
				Spec: cmapi.CertificateRequestSpec{
					Username: "example",
					IssuerRef: cmmeta.ObjectReference{
						Name:  "test-name",
						Kind:  "test-kind",
						Group: "test-group",
					},
				},
			})

			assert.Equalf(t, test.expErr, err != nil, "%v", err)
			assert.Equal(t, test.expResponse, response)
		})
	}
}
