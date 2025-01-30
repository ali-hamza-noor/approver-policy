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

package predicate

import (
	"context"
	"path"
	"testing"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyapi "github.com/cert-manager/approver-policy/pkg/apis/policy/v1alpha1"
	testenv "github.com/cert-manager/approver-policy/test/env"
)

func Test_RBACBound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(func() {
		cancel()
	})

	env := testenv.RunControlPlane(t, ctx,
		testenv.GetenvOrFail(t, "CERT_MANAGER_CRDS"),
		path.Join("..", "..", "..", "..", "..", "deploy", "crds"),
	)

	const (
		requestUser      = "example"
		requestNamespace = "test-namespace"
	)

	if err := env.AdminClient.Create(context.TODO(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: requestNamespace}},
	); err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		apiObjects  []client.Object
		policies    []policyapi.CertificateRequestPolicy
		expPolicies []policyapi.CertificateRequestPolicy
	}{
		"if no CertificateRequestPolicies exist, return nothing": {
			apiObjects:  nil,
			policies:    nil,
			expPolicies: nil,
		},
		"if no CertificateRequestPolicies are bound to the user, return ResultUnprocessed": {
			apiObjects: []client.Object{
				&policyapi.CertificateRequestPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
			policies:    nil,
			expPolicies: nil,
		},
		"if single CertificateRequestPolicy exists but not bound, return nothing": {
			apiObjects: []client.Object{},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
					IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
				}},
			}},
			expPolicies: nil,
		},
		"if multiple CertificateRequestPolicy exists but not bound, return nothing": {
			apiObjects: []client.Object{},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
			expPolicies: nil,
		},
		"if single CertificateRequestPolicy bound at cluster level, return policy": {
			apiObjects: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-binding"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"}, Verbs: []string{"use"}, ResourceNames: []string{"test-policy-a"}},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "test-binding"},
				},
			},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
					IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
				}},
			}},
			expPolicies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
					IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
				}},
			}},
		},
		"if single CertificateRequestPolicy bound at namespace, return policy": {
			apiObjects: []client.Object{
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-binding"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"}, Verbs: []string{"use"}, ResourceNames: []string{"test-policy-a"}},
					},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-binding"},
				},
			},
			policies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
					IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
				}},
			}},
			expPolicies: []policyapi.CertificateRequestPolicy{{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
				Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
					IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
				}},
			}},
		},
		"if two CertificateRequestPolicies bound at cluster level, return policies": {
			apiObjects: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-binding"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"},
							Verbs: []string{"use"}, ResourceNames: []string{"test-policy-a", "test-policy-b"},
						},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "test-binding"},
				},
			},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
		},
		"if two CertificateRequestPolicies bound at namespace level, return policies": {
			apiObjects: []client.Object{
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-binding"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"},
							Verbs: []string{"use"}, ResourceNames: []string{"test-policy-a", "test-policy-b"},
						},
					},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-binding"},
				},
			},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
		},
		"if two CertificateRequestPolicies bound at namespace and cluster, return policies": {
			apiObjects: []client.Object{
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-binding-namespaced"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"},
							Verbs: []string{"use"}, ResourceNames: []string{"test-policy-a"},
						},
					},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-binding-namespaced"},
				},
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-binding-cluster"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"}, Verbs: []string{"use"}, ResourceNames: []string{"test-policy-b"}},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "test-binding-cluster"},
				},
			},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
		},
		"if two CertificateRequestPolicies bound at namespace and cluster and other policies exist, return only bound policies": {
			apiObjects: []client.Object{
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-binding-namespaced"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"},
							Verbs: []string{"use"}, ResourceNames: []string{"test-policy-a"},
						},
					},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace, Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-binding-namespaced"},
				},
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-binding-cluster"},
					Rules: []rbacv1.PolicyRule{
						{APIGroups: []string{"policy.cert-manager.io"}, Resources: []string{"certificaterequestpolicies"}, Verbs: []string{"use"}, ResourceNames: []string{"test-policy-b"}},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
					Subjects:   []rbacv1.Subject{{Kind: "User", Name: requestUser, APIGroup: "rbac.authorization.k8s.io"}},
					RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "test-binding-cluster"},
				},
			},
			policies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-c"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-d"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-a"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy-b"},
					Spec: policyapi.CertificateRequestPolicySpec{Selector: policyapi.CertificateRequestPolicySelector{
						IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{},
					}},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() {
				for _, obj := range test.apiObjects {
					if err := env.AdminClient.Delete(context.TODO(), obj); err != nil {
						// Don't Fatal here as a ditch effort to at least try to clean-up
						// everything.
						t.Errorf("failed to deleted existing object: %s", err)
					}
				}
			})

			for _, obj := range test.apiObjects {
				if err := env.AdminClient.Create(context.TODO(), obj); err != nil {
					t.Fatalf("failed to create new object: %s", err)
				}
			}

			req := &cmapi.CertificateRequest{
				ObjectMeta: metav1.ObjectMeta{Namespace: requestNamespace},
				Spec: cmapi.CertificateRequestSpec{
					Username: "example",
					IssuerRef: cmmeta.ObjectReference{
						Name:  "test-name",
						Kind:  "test-kind",
						Group: "test-group",
					},
				},
			}
			policies, err := RBACBound(env.AdminClient)(context.TODO(), req, test.policies)
			assert.NoError(t, err)
			assert.Equal(t, test.expPolicies, policies)
		})
	}
}

func Test_Ready(t *testing.T) {
	tests := map[string]struct {
		policies    []policyapi.CertificateRequestPolicy
		expPolicies []policyapi.CertificateRequestPolicy
	}{
		"no given policies should return no policies": {
			policies:    nil,
			expPolicies: nil,
		},
		"single policy with no conditions should return no policies": {
			policies:    []policyapi.CertificateRequestPolicy{},
			expPolicies: nil,
		},
		"single policy with ready condition false should return no policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionFalse},
				}}},
			},
			expPolicies: nil,
		},
		"single policy with ready condition true should return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
				}}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
				}}},
			},
		},
		"one policy which is ready another not, return single policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionFalse},
				}}},
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
				}}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
				}}},
			},
		},
		"mix of different conditions including ready should return only ready policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionFalse},
					{Type: "C", Status: corev1.ConditionTrue},
				}}},
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
					{Type: "B", Status: corev1.ConditionTrue},
				}}},
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
					{Type: "A", Status: corev1.ConditionTrue},
				}}},
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
				}}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
					{Type: "B", Status: corev1.ConditionTrue},
				}}},
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
					{Type: "A", Status: corev1.ConditionTrue},
				}}},
				{Status: policyapi.CertificateRequestPolicyStatus{Conditions: []policyapi.CertificateRequestPolicyCondition{
					{Type: policyapi.CertificateRequestPolicyConditionReady, Status: corev1.ConditionTrue},
				}}},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			policies, err := Ready(context.TODO(), nil, test.policies)
			assert.NoError(t, err)
			if !apiequality.Semantic.DeepEqual(test.expPolicies, policies) {
				t.Errorf("unexpected policies returned:\nexp=%#+v\ngot=%#+v", test.expPolicies, policies)
			}
		})
	}
}

func Test_SelectorIssuerRef(t *testing.T) {
	baseRequest := &cmapi.CertificateRequest{
		Spec: cmapi.CertificateRequestSpec{
			IssuerRef: cmmeta.ObjectReference{
				Name:  "test-name",
				Kind:  "test-kind",
				Group: "test-group",
			},
		},
	}

	tests := map[string]struct {
		request     *cmapi.CertificateRequest
		policies    []policyapi.CertificateRequestPolicy
		expPolicies []policyapi.CertificateRequestPolicy
	}{
		"if no policies given, return no policies": {
			policies:    nil,
			expPolicies: nil,
		},
		"if policy specifies cert-manager defaults and request omits defaults, return policy": {
			request: &cmapi.CertificateRequest{Spec: cmapi.CertificateRequestSpec{IssuerRef: cmmeta.ObjectReference{
				Name: "my-issuer",
			}}},
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("my-issuer"), Kind: ptr.To("Issuer"), Group: ptr.To("cert-manager.io"),
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("my-issuer"), Kind: ptr.To("Issuer"), Group: ptr.To("cert-manager.io"),
					}},
				}},
			},
		},
		"if policy given that doesn't match, return no policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name"), Kind: ptr.To("kind"), Group: ptr.To("group"),
					}},
				}},
			},
			expPolicies: nil,
		},
		"if two policies given that doesn't match, return no policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name"), Kind: ptr.To("kind"), Group: ptr.To("group"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name-2"), Kind: ptr.To("kind-2"), Group: ptr.To("group-2"),
					}},
				}},
			},
			expPolicies: nil,
		},
		"if one of two policies match all with all nils, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: new(policyapi.CertificateRequestPolicySelectorIssuerRef)},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name"), Kind: ptr.To("kind"), Group: ptr.To("group"),
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: new(policyapi.CertificateRequestPolicySelectorIssuerRef)},
				}},
			},
		},
		"if one of two policies match all with wildcard, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name"), Kind: ptr.To("kind"), Group: ptr.To("group"),
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
			},
		},
		"if both of two policies match all with empty, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: new(policyapi.CertificateRequestPolicySelectorIssuerRef)},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: new(policyapi.CertificateRequestPolicySelectorIssuerRef)},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: new(policyapi.CertificateRequestPolicySelectorIssuerRef)},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: new(policyapi.CertificateRequestPolicySelectorIssuerRef)},
				}},
			},
		},
		"if both of two policies match all with wildcard, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
			},
		},
		"if one policy matches with, other doesn't, return 1": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("test-name"), Kind: ptr.To("test-kind"), Group: ptr.To("test-group"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name"), Kind: ptr.To("kind"), Group: ptr.To("group"),
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("test-name"), Kind: ptr.To("test-kind"), Group: ptr.To("test-group"),
					}},
				}},
			},
		},
		"if some polices match with a mix of exact, just wildcard and mix return policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("test-name"), Kind: ptr.To("test-kind"), Group: ptr.To("test-group"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("name"), Kind: ptr.To("kind"), Group: ptr.To("group"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("test-*"), Kind: ptr.To("*-kind"), Group: ptr.To("*up"),
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("test-name"), Kind: ptr.To("test-kind"), Group: ptr.To("test-group"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("*"), Kind: ptr.To("*"), Group: ptr.To("*"),
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{IssuerRef: &policyapi.CertificateRequestPolicySelectorIssuerRef{
						Name: ptr.To("test-*"), Kind: ptr.To("*-kind"), Group: ptr.To("*up"),
					}},
				}},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.request == nil {
				test.request = baseRequest
			}
			policies, err := SelectorIssuerRef(context.TODO(), test.request, test.policies)
			assert.NoError(t, err)
			if !apiequality.Semantic.DeepEqual(test.expPolicies, policies) {
				t.Errorf("unexpected policies returned:\nexp=%#+v\ngot=%#+v", test.expPolicies, policies)
			}
		})
	}
}

func Test_SelectorNamespace(t *testing.T) {
	var (
		baseRequest = &cmapi.CertificateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
			},
		}
		testns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"}}
	)

	tests := map[string]struct {
		policies          []policyapi.CertificateRequestPolicy
		existingNamespace runtime.Object
		expPolicies       []policyapi.CertificateRequestPolicy
		expErr            bool
	}{
		"if namespace for request doesn't exist and using match names, expect no error": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"bar"},
					}},
				}},
			},
			existingNamespace: nil,
			expPolicies:       nil,
			expErr:            false,
		},
		"if namespace for request doesn't exist and using match labels, expect error": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
			},
			existingNamespace: nil,
			expPolicies:       nil,
			expErr:            true,
		},
		"if no policies given, return no policies": {
			policies:          nil,
			existingNamespace: testns,
			expPolicies:       nil,
			expErr:            false,
		},
		"if policy given that doesn't match namespace match name, return no policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"foo"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies:       nil,
			expErr:            false,
		},
		"if policy given that doesn't match namespace match labels, return no policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
			},
			existingNamespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace", Labels: map[string]string{"bar": "foo"}}},
			expPolicies:       nil,
			expErr:            false,
		},
		"if two policies given that doesn't match, return no policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"foo"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
			},
			existingNamespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace", Labels: map[string]string{"bar": "foo"}}},
			expPolicies:       nil,
			expErr:            false,
		},
		"if one of two policies match all with all nils, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: new(policyapi.CertificateRequestPolicySelectorNamespace)},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"foo"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: new(policyapi.CertificateRequestPolicySelectorNamespace)},
				}},
			},
			expErr: false,
		},
		"if one of two policies match all with wildcard, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"foo"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
			},
			expErr: false,
		},
		"if one of two policies match with wildcard suffix, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"test-*"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"foo"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"test-*"},
					}},
				}},
			},
			expErr: false,
		},
		"if one of two policies match with wildcard prefix, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*-namespace"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"foo"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*-namespace"},
					}},
				}},
			},
			expErr: false,
		},
		"if both of two policies match all with empty, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: new(policyapi.CertificateRequestPolicySelectorNamespace)},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: new(policyapi.CertificateRequestPolicySelectorNamespace)},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: new(policyapi.CertificateRequestPolicySelectorNamespace)},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: new(policyapi.CertificateRequestPolicySelectorNamespace)},
				}},
			},
			expErr: false,
		},
		"if both of two policies match all with wildcard, return policy": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
			},
			expErr: false,
		},
		"if one policy matches with, other doesn't, return 1": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"test-namespace"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
			},
			existingNamespace: testns,
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"test-namespace"},
					}},
				}},
			},
			expErr: false,
		},
		"if some polices match with a mix of exact, just wildcard and mix return policies": {
			policies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"test-namespace"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"bar": "foo"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames:  []string{"test-namespace"},
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames:  []string{"namespace-test"},
						MatchLabels: map[string]string{"bar": "foo"},
					}},
				}},
			},
			expPolicies: []policyapi.CertificateRequestPolicy{
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"test-namespace"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames: []string{"*"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
				{Spec: policyapi.CertificateRequestPolicySpec{
					Selector: policyapi.CertificateRequestPolicySelector{Namespace: &policyapi.CertificateRequestPolicySelectorNamespace{
						MatchNames:  []string{"test-namespace"},
						MatchLabels: map[string]string{"foo": "bar"},
					}},
				}},
			},
			existingNamespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace", Labels: map[string]string{"foo": "bar"}}},
			expErr:            false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fakeclient.NewClientBuilder().
				WithScheme(policyapi.GlobalScheme)
			if test.existingNamespace != nil {
				builder = builder.WithRuntimeObjects(test.existingNamespace)
			}
			fakeclient := builder.Build()

			policies, err := SelectorNamespace(fakeclient)(context.TODO(), baseRequest, test.policies)
			assert.Equal(t, err != nil, test.expErr, "%v", err)
			if !test.expErr && !apiequality.Semantic.DeepEqual(test.expPolicies, policies) {
				t.Errorf("unexpected policies returned:\nexp=%#+v\ngot=%#+v", test.expPolicies, policies)
			}
		})
	}
}
