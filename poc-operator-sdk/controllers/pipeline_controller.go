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
	"bufio"
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pipelinev1alpha1 "github.com/viniciusCSreis/poc-operator-sdk/api/v1alpha1"
)

const PendingPhase = "Pending"
const RunningPhase = "Running"
const SucceededPhase = "Succeeded"
const FailedPhase = "Failed"
const controllerLabel = "controller_name"

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	K8sClient *kubernetes.Clientset
	Scheme    *runtime.Scheme
}

//+kubebuilder:rbac:groups=pipeline.example.com,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pipeline.example.com,resources=pipelines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pipeline.example.com,resources=pipelines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile loop")

	// Fetch the Pipeline instance
	pipeline := &pipelinev1alpha1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get Pipeline")
		return ctrl.Result{}, err
	}

	// Check if the pod already exists, if not create a new one
	pod := &v1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: pipeline.Name, Namespace: pipeline.Namespace}, pod)
	if err != nil && errors.IsNotFound(err) {
		err = r.createPod(ctx, pipeline)
		if err != nil {
			return ctrl.Result{}, err
		}
		// New Pod created successfully - return and requeue after 1s
		return ctrl.Result{RequeueAfter: time.Second}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("pod %s have status %s", pod.Name, pod.Status.Phase))

	switch pod.Status.Phase {
	case RunningPhase:
		err = r.processRunning(ctx, pipeline)
	case SucceededPhase:
		err = r.processSuccess(ctx, pipeline, pod)
	case FailedPhase:
		err = r.processFailed(ctx, pipeline)
	}
	return ctrl.Result{}, err

}

func (r *PipelineReconciler) createPod(ctx context.Context, pipeline *pipelinev1alpha1.Pipeline) error {

	logger := log.FromContext(ctx)

	newPod := r.podForPipeline(pipeline)
	logger.Info("Creating a new Pod", "Pod.Namespace", newPod.Namespace, "Pod.Name", newPod.Name)
	err := r.Create(ctx, newPod)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(
			err,
			"Failed to create new pod",
			"Pod.Namespace", newPod.Namespace,
			"Pod.Name", newPod.Name,
		)
		return err
	}

	pipeline.Status.Phase = PendingPhase
	logger.Info(fmt.Sprintf("Updating pipeline.Status.Phase to %s", pipeline.Status.Phase))
	if err := r.Status().Update(ctx, pipeline); err != nil {
		logger.Error(err, "fail to update status")
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pipelinev1alpha1.Pipeline{}).
		Owns(&v1.Pod{}).
		Complete(r)
}

func (r *PipelineReconciler) podForPipeline(pipeline *pipelinev1alpha1.Pipeline) *v1.Pod {

	deadlineSeconds := int64(pipeline.Spec.Timeout)
	terminationGracePeriodSeconds := int64(0)

	containerEnvs := make([]v1.EnvVar, 0)

	for _, input := range pipeline.Spec.Envs {
		containerEnvs = append(containerEnvs, v1.EnvVar{
			Name:  input.Name,
			Value: input.Value,
		})
	}

	podSec := v1.PodSpec{
		ActiveDeadlineSeconds:         &deadlineSeconds,
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		Containers: []v1.Container{
			{
				Name:  "generic-dockerimage",
				Image: "generic-dockerimage:local",
				Env:   containerEnvs,
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.Name,
			Namespace: pipeline.Namespace,
			Labels: map[string]string{
				controllerLabel: pipeline.Name,
			},
		},
		Spec: podSec,
	}
	_ = controllerutil.SetControllerReference(pipeline, pod, r.Scheme)
	return pod
}

func (r *PipelineReconciler) processRunning(
	ctx context.Context, pipeline *pipelinev1alpha1.Pipeline,
) error {
	logger := log.FromContext(ctx)
	if pipeline.Status.Phase != PendingPhase {
		return nil
	}
	pipeline.Status.Phase = RunningPhase
	logger.Info(fmt.Sprintf("Updating pipeline.Status.Phase to %s", pipeline.Status.Phase))
	if err := r.Status().Update(ctx, pipeline); err != nil {
		logger.Error(err, "fail to update status")
		return err
	}
	return nil
}

func (r *PipelineReconciler) processSuccess(
	ctx context.Context, pipeline *pipelinev1alpha1.Pipeline, pod *v1.Pod,
) error {
	logger := log.FromContext(ctx)

	if pipeline.Status.Phase == SucceededPhase {
		return nil
	}

	logs, err := r.getPodLogs(ctx, pod)
	if err != nil {
		logger.Error(err, "fail to get pod logs")
		return err
	}

	pipeline.Status.Phase = SucceededPhase
	pipeline.Status.Logs = logs
	logger.Info(fmt.Sprintf("Updating pipeline.Status.Phase to %s", pipeline.Status.Phase))
	logger.Info(fmt.Sprintf("Updating pipeline.Status.Logs to %s", pipeline.Status.Logs))
	if err := r.Status().Update(ctx, pipeline); err != nil {
		logger.Error(err, "fail to update status")
		return err
	}
	return nil
}

func (r *PipelineReconciler) processFailed(
	ctx context.Context, pipeline *pipelinev1alpha1.Pipeline,
) error {
	logger := log.FromContext(ctx)

	if pipeline.Status.Phase == FailedPhase {
		return nil
	}
	pipeline.Status.Phase = FailedPhase
	logger.Info(fmt.Sprintf("Updating pipeline.Status.Phase to %s", pipeline.Status.Phase))
	if err := r.Status().Update(ctx, pipeline); err != nil {
		logger.Error(err, "fail to update status")
		return err
	}
	return nil
}

func (r *PipelineReconciler) getPodLogs(ctx context.Context, pod *v1.Pod) (string, error) {
	opts := v1.PodLogOptions{
		Follow: true,
	}
	req := r.K8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &opts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}

	logs := ""
	scanner := bufio.NewScanner(podLogs)
	for scanner.Scan() {
		msg := scanner.Text() + "\n"
		logs += msg
	}
	return logs, nil
}
