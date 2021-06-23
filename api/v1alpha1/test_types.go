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
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TestSpec defines the desired state of Test
type TestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Should the tester start a blocking test or not? Default true
	// +optional
	Blocking bool `json:"blocking,omitempty"`

	// Certificate
	// +optional
	Certificate *Certificate `json:"certificate,omitempty"`

	// Tekton creates a tekton tester that will check the status of a triggered Tekton PipelineRun.
	// +optional
	Tekton *TektonTest `json:"tekton,omitempty"`

	// Webhook creates a webhook tester which will host a endpoint to receive callBack webhooks.
	// +optional
	Webhook *WebhookTest `json:"webhook,omitempty"`
}

type Certificate struct {
	// SecretRef holds the name to a secret that contains a Kubernetes TLS secret.
	// +required
	SecretRef meta.LocalObjectReference `json:"secretRef,omitempty"`
}

type TektonTest struct {
	// The namespace where we will find the Tekton PipelineRun, defaults to the namespace the CR is created in.
	// +optional
	Namespace string `json:"namespace"`

	// The Tekton eventListener URL
	// +required
	Url string `json:"url"`
}

// +kubebuilder:validation:Enum=grpc;http;http2;tcp
type PortProtocol string

const (
	PortProtocolHTTP  PortProtocol = "http"
	PortProtocolHTTP2 PortProtocol = "http2"
)

type WebhookTest struct {
	// content-type header, defaults "application/json".
	// +optional
	ContentType string `json:"contentType,omitempty"`

	// Custom headers to add to your webhook
	// +optional
	Headers *map[string]string `json:"headers,omitempty"`

	// host header, defaults to the deployment service-name.
	// +optional
	Host string `json:"host,omitempty"`

	// What protocol to use when sending the webhook.
	// Valid values are:
	// - http (default)
	// - http2
	// +optional
	Protocol PortProtocol `json:"protocol,omitempty"`

	// A token to forward with your webhook.
	// +optional
	Token string `json:"token,omitempty"`

	// userAgent header, default "flagger-tester/1.0.0".
	// +optional
	UserAgent string `json:"userAgent,omitempty"`

	// Url where to send the webhook
	// +required
	Url string `json:"url,omitempty"`
}

// TestStatus defines the observed state of Test
type TestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Test is the Schema for the tests API
type Test struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestSpec   `json:"spec,omitempty"`
	Status TestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TestList contains a list of Test
type TestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Test `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Test{}, &TestList{})
}
