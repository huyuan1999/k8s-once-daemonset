/*
Copyright 2022.

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

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OnceDaemonSetSpec defines the desired state of OnceDaemonSet
type OnceDaemonSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of OnceDaemonSet. Edit oncedaemonset_types.go to remove/update

}

// OnceDaemonSetStatus defines the observed state of OnceDaemonSet
type OnceDaemonSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// DesiredNumberScheduled int `json:"desiredNumberScheduled"` // 表示集群中需要部署 ods pod 的节点数量(需要运行的节点)
	// NumberCompleted        int `json:"numberCompleted"`        // Completed 状态的 pod 数
	// NumberAvailable        int `json:"numberAvailable"`        // 活跃的 pod 数（Running）
	// NumberUnavailable      int `json:"numberUnavailable"`      // 非活跃的 pod 数（Failed,Unknown,Error）
}

/*
////+kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.numberAvailable",description="available"
////+kubebuilder:printcolumn:name="Failed",type="integer",JSONPath=".status.numberUnavailable",description="failed"
////+kubebuilder:printcolumn:name="Completed",type="integer",JSONPath=".status.numberCompleted",description="completed"
////+kubebuilder:printcolumn:name="DesiredNumberScheduled",type="integer",priority=1,JSONPath=".status.desiredNumberScheduled",description="completed"
*/

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OnceDaemonSet is the Schema for the oncedaemonsets API
type OnceDaemonSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   v1.PodSpec          `json:"spec,omitempty"`
	Status OnceDaemonSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OnceDaemonSetList contains a list of OnceDaemonSet
type OnceDaemonSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OnceDaemonSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OnceDaemonSet{}, &OnceDaemonSetList{})
}
