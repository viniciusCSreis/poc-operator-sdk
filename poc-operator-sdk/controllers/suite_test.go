/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	pipelinev1alpha1 "github.com/viniciusCSreis/poc-operator-sdk/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	timeout   = time.Second * 10
	interval  = time.Millisecond * 250
	namespace = "default"
)

var ctx = context.Background()
var k8sClient client.Client
var testEnv *envtest.Environment
var mgr ctrl.Manager

//to run using make use: make test
func TestController_success_integration(t *testing.T) {

	tests := []struct {
		name  string
		logic func(t *testing.T)
	}{
		{
			name:  "Run with success",
			logic: runWithSuccess,
		},
		{
			name:  "Handle FailedPhase",
			logic: handleFailedPhase,
		},
	}

	startEnvTest()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.logic(t)
		})
	}

	stopEnvTest()

}

func handleFailedPhase(t *testing.T) {

	pipelineName := "test-pipeline-" + uuid.New().String()

	pipeline := createPipeline(pipelineName, namespace)
	err := k8sClient.Create(ctx, pipeline)
	if err != nil {
		panic(err)
	}

	pipelineLookupKey := types.NamespacedName{Name: pipelineName, Namespace: namespace}
	createdPipeline := &pipelinev1alpha1.Pipeline{}

	//Wait Status Pending
	wait(t, func() bool {
		err := k8sClient.Get(ctx, pipelineLookupKey, createdPipeline)
		if err != nil || createdPipeline.Status.Phase != PendingPhase {
			return false
		}
		return true
	}, timeout, interval)

	//Wait Pod to be created
	podLookupKey := types.NamespacedName{Name: pipelineName, Namespace: namespace}
	createdPod := &v1.Pod{}
	wait(t, func() bool {
		err := k8sClient.Get(ctx, podLookupKey, createdPod)
		return err == nil
	}, timeout, interval)

	testLabel(t, createdPod.Labels, controllerLabel, pipelineName)

	//Change Pod Status.Phase Failed
	createdPod.Status.Phase = FailedPhase
	err = k8sClient.Status().Update(ctx, createdPod)
	if err != nil {
		panic(err)
	}

	//Wait Pipeline Phase Failed
	wait(t, func() bool {
		err := k8sClient.Get(ctx, pipelineLookupKey, createdPipeline)
		if err != nil || createdPipeline.Status.Phase != FailedPhase {
			return false
		}
		return true
	}, timeout, interval)
}

func runWithSuccess(t *testing.T) {

	pipelineName := "test-pipeline-" + uuid.New().String()

	pipeline := createPipeline(pipelineName, namespace)
	err := k8sClient.Create(ctx, pipeline)
	if err != nil {
		panic(err)
	}

	pipelineLookupKey := types.NamespacedName{Name: pipelineName, Namespace: namespace}
	createdPipeline := &pipelinev1alpha1.Pipeline{}

	//Wait Status Pending
	wait(t, func() bool {
		err := k8sClient.Get(ctx, pipelineLookupKey, createdPipeline)
		if err != nil || createdPipeline.Status.Phase != PendingPhase {
			return false
		}
		return true
	}, timeout, interval)

	//Wait Pod to be created
	podLookupKey := types.NamespacedName{Name: pipelineName, Namespace: namespace}
	createdPod := &v1.Pod{}
	wait(t, func() bool {
		err := k8sClient.Get(ctx, podLookupKey, createdPod)
		return err == nil
	}, timeout, interval)

	testLabel(t, createdPod.Labels, controllerLabel, pipelineName)

	//Change pod Phase to Running
	createdPod.Status.Phase = RunningPhase
	err = k8sClient.Status().Update(ctx, createdPod)
	if err != nil {
		panic(err)
	}

	//Wait Pipeline Phase Running
	wait(t, func() bool {
		err := k8sClient.Get(ctx, pipelineLookupKey, createdPipeline)
		if err != nil || createdPipeline.Status.Phase != RunningPhase {
			return false
		}
		return true
	}, timeout, interval)

	//Change pod Phase to Succeed
	createdPod.Status.Phase = SucceededPhase
	err = k8sClient.Status().Update(ctx, createdPod)
	if err != nil {
		panic(err)
	}

	//Wait Pipeline Phase Running
	wait(t, func() bool {
		err := k8sClient.Get(ctx, pipelineLookupKey, createdPipeline)
		if err != nil || createdPipeline.Status.Phase != SucceededPhase {
			return false
		}
		return true
	}, timeout, interval)
}

func stopEnvTest() {
	err := testEnv.Stop()
	if err != nil {
		//https://github.com/kubernetes-sigs/controller-runtime/issues/1571
		fmt.Printf("Err:%v", err.Error())
	}
	time.Sleep(time.Second * 5)
}

func startEnvTest() {
	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}
	if cfg == nil {
		panic("cfg is nil")
	}

	err = pipelinev1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}
	if k8sClient == nil {
		panic("k8sClient is nil")
	}

	mgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     ":0",
		Port:                   9443,
		HealthProbeBindAddress: ":0",
		LeaderElection:         false,
		LeaderElectionID:       "7f0c45a6.example.com",
	})
	if err != nil {
		panic(err)
	}

	k8sClientSet := kubernetes.NewForConfigOrDie(cfg)

	controller := &PipelineReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		K8sClient: k8sClientSet,
	}

	err = controller.SetupWithManager(mgr)
	if err != nil {
		panic(err)
	}

	go func() {
		err = mgr.Start(ctx)
		if err != nil {
			panic(err)
		}
	}()
}

func wait(t *testing.T, logic func() bool, timeout, interval time.Duration) {
	if logic() == true {
		return
	}

	timeoutTime := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if logic() == true {
			return
		}
		if time.Now().After(timeoutTime) {
			break
		}
	}

	t.Helper()
	t.Errorf("timeout error")

}

func createPipeline(name, namespace string) *pipelinev1alpha1.Pipeline {
	return &pipelinev1alpha1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pipeline.example.com/v1alpha1",
			Kind:       "Pipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: pipelinev1alpha1.PipelineSpec{
			Envs: []pipelinev1alpha1.PipelineEnvs{
				{
					Name:  "MSG_TO_PRINT",
					Value: "BLA_BLOW",
				},
			},

			Timeout: 30,
		},
	}
}

func testLabel(t *testing.T, labels map[string]string, labelName string, want string) {
	if labels[labelName] != want {
		t.Helper()
		t.Errorf("Wrong label %s got: %s want: %s",
			labelName, labels[labelName], want)
	}
}
