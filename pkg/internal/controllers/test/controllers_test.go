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

package test

import (
	"path"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	testenv "github.com/cert-manager/approver-policy/test/env"
)

// Test_Controllers runs the full suite of tests for the approver-policy
// controllers.
func Test_Controllers(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	env = testenv.RunControlPlane(t, t.Context(),
		testenv.GetenvOrFail(t, "CERT_MANAGER_CRDS"),
		path.Join("..", "..", "..", "..", "deploy", "crds"),
	)

	ginkgo.RunSpecs(t, "approver-policy-controllers")
}
