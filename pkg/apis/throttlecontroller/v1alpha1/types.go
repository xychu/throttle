/*
Copyright 2017 The Kubernetes Authors.

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
	coreapi "k8s.io/kubernetes/pkg/apis/core"
)

// The following identify resource constants for Kubernetes object types
const (
	// Nvidia GPU resource, in devices.
	ResourceGPU coreapi.ResourceName = "nvidia.com/gpu"
	// GPU request, in devices.
	ResourceRequestsGPU coreapi.ResourceName = "requests.gpu"
	// GPU limit, in cores.
	ResourceLimitsGPU coreapi.ResourceName = "limits.gpu"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GPUQuota is a specification for a GPUQuota resource
type GPUQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   coreapi.ResourceQuotaSpec   `json:"spec"`
	Status coreapi.ResourceQuotaStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GPUQuotaList is a list of GPUQuota resources
type GPUQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GPUQuota `json:"items"`
}
