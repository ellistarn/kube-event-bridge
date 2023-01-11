/*
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

// SlackTargetSpec describes the desired state of the SlackTarget
type SlackTargetSpec struct {
	// EventRule is the name of the event rule to source messages
	// +required
	EventRule string `json:"eventRule"`
	// HTTPEndpoint of the slack webhook to post the messages
	// +required
	HTTPEndpoint string `json:"httpEndpoint"`
}

// SlackTarget is the Schema for the SlackTargets API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=slacktargets,scope=Cluster
type SlackTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SlackTargetSpec `json:"spec,omitempty"`
}

// SlackTargetList contains a list of Provisioner
// +kubebuilder:object:root=true
type SlackTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SlackTarget `json:"items"`
}
