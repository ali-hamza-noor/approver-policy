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

package env

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getCMValidatingWebhookConfig returns a ValidatingWebhookConfiguration object
// for the cert-manager webhook. url should be the URL that the webhook process
// is reachable from, and caPEM the CA of the certificate it is serving from.
func getCMValidatingWebhookConfig(url string, caPEM []byte) client.Object {
	failurePolicy := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone
	validateURL := fmt.Sprintf("%s/validate", url)
	webhook := admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind: "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cert-manager-webhook",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "webhook.cert-manager.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      &validateURL,
					CABundle: caPEM,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"cert-manager.io", "acme.cert-manager.io"},
							APIVersions: []string{"*"},
							Resources:   []string{"*/*"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	return &webhook
}

// getCMMutatingWebhookConfig returns a MutatingWebhookConfiguration object for
// the cert-manager webhook. url should be the URL that the webhook process is
// reachable from, and caPEM the CA of the certificate it is serving from.
func getCMMutatingWebhookConfig(url string, caPEM []byte) client.Object {
	failurePolicy := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone
	validateURL := fmt.Sprintf("%s/mutate", url)
	webhook := admissionregistrationv1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind: "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cert-manager-webhook",
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name: "webhook.cert-manager.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      &validateURL,
					CABundle: caPEM,
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"cert-manager.io", "acme.cert-manager.io"},
							APIVersions: []string{"*"},
							Resources:   []string{"*/*"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	return &webhook
}
