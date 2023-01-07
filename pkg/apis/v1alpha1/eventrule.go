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

// EventRuleSpec describes the desired state of the EventRule
type EventRuleSpec struct {
	// EventBus to send the messages to. Defaults to `default`.
	// +optional
	EventBus *string  `json:"eventBus,omitempty"`
	Filter   []Filter `json:"filter,omitempty"`
}

type Filter struct {
	// Reason of the event. Matches all, if unset
	// +optional
	Reason *string `json:"reason,omitempty"`
	// Type of the message: (Info | Warning | Error). Matches all, if unset
	// +optional
	Type *string `json:"level,omitempty"`
}

// EventRule is the Schema for the EventRules API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=eventrules,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type EventRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EventRuleSpec   `json:"spec,omitempty"`
	Status EventRuleStatus `json:"status,omitempty"`
}

// EventRuleList contains a list of Provisioner
// +kubebuilder:object:root=true
type EventRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventRule `json:"items"`
}
