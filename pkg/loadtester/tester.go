/*
Copyright 2020 The Flux authors

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

package loadtester

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	testingv1alpha1 "flagger.app/testing/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TaskTypeTester = "tester"

type TesterTask struct {
	TaskBase
	name         string
	kubeClient   client.Client
	logCmdOutput bool
}

func (task *TesterTask) Hash() string {
	return hash(task.canary + task.name)
}

func (task *TesterTask) Run(ctx context.Context) (*TaskRunResult, error) {
	tst, err := task.checkConfig(ctx)
	if err != nil {
		return &TaskRunResult{
			ok:  false,
			out: []byte(fmt.Sprintf("Unable to find tester CR %v", task.name)),
		}, err
	}

	httpResp, err := task.httpPost(ctx, tst)
	if err != nil {
		return &TaskRunResult{
			ok:  false,
			out: []byte("Unable to reach tekton endpoint"),
		}, err
	}

	// TODO double check the status code of tekton return
	if httpResp.StatusCode != http.StatusCreated {
		return &TaskRunResult{
			ok: false,
			//TODO would be nice to add the HttpBody, should we also add a log?
			out: []byte("Unable to create a new tekton Task"),
		}, fmt.Errorf("Tekton enventListener gave %d", httpResp.StatusCode)
	}

	// TODO(EDVIN) import the tekton eventListerner API
	/* Parse the output
	Get the eventListener ID
	Check when the pipelineRun using the eventListener ID, some kube ctl ninja stuff using watchers
	When it passes return ok, else return error. Should probably use the tst timeout value
	*/

	// task.logger.With("canary", task.canary).Infof("command finished %s", task.command)

	return &TaskRunResult{
		ok: true,
		// TODO use the output from the pipelineRun status
		out: []byte("Task status ok"),
	}, err

}

func (task *TesterTask) String() string {
	return task.name
}

func (task *TesterTask) httpPost(ctx context.Context, tst *testingv1alpha1.Test) (*http.Response, error) {

	// TODO is there some logic needed to manage http vs https?
	// TODO here we should read the TLS secret
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{
		Transport: tr,
	}

	// TODO the timeout should be from tst increase tst with this information.
	ctxTimeout, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(5*time.Second)))
	defer cancel()

	// TODO set correct Body
	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodPost, tst.Spec.Tekton.Url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return resp, nil
}

// checkConfig TODO(Edvin) this might be possible to use in more places, use here for now
func (task *TesterTask) checkConfig(ctx context.Context) (*testingv1alpha1.Test, error) {
	var tst testingv1alpha1.Test
	var allTst testingv1alpha1.TestList
	var name string
	var namespace string
	err := task.kubeClient.List(ctx, &allTst)
	if err != nil {
		return nil, err
	}

	for _, oneTst := range allTst.Items {
		name = oneTst.GetName()
		namespace = oneTst.GetNamespace()
		if name == task.name {
			break
		}
	}
	tstNamespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	err = task.kubeClient.Get(ctx, tstNamespacedName, &tst)
	if err != nil {
		return nil, err
	}
	return &tst, err
}
