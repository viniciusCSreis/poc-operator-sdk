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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineEnvs struct {
	//Name env name
	Name string `json:"name"`
	//Value env value
	Value string `json:"value"`
}

// PipelineSpec defines the desired state of Pipeline
type PipelineSpec struct {
	//Envs to run pipeline
	Envs []PipelineEnvs `json:"envs"`
	//Timeout pipeline timeout in seconds
	Timeout int `json:"timeout"`
}

// PipelineStatus defines the observed state of Pipeline
type PipelineStatus struct {
	// Phase pipeline phase: [pending, running, completed]
	Phase string `json:"phase"`
	// Logs logs of a finished pipeline
	Logs string `json:"logs"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Pipeline is the Schema for the pipelines API
type Pipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineSpec   `json:"spec,omitempty"`
	Status PipelineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pipeline{}, &PipelineList{})
}
