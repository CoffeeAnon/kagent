// Copyright 2026 The kagent Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha3

import corev1 "k8s.io/api/core/v1"

// RuntimeEnvVar configures one literal runtime environment variable.
type RuntimeEnvVar struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// Value is a literal value, including an empty string.
	// +required
	Value string `json:"value"`

	// Deferred until secret-backed environment injection is supported.
	// CredentialRef *corev1.SecretKeySelector `json:"credentialRef,omitempty"`
}

// RuntimeSnapshotPolicy configures storage for Substrate snapshots.
type RuntimeSnapshotPolicy struct {
	// Location is the snapshot storage location used by Substrate.
	// +kubebuilder:validation:Pattern=`^[^[:space:]]+$`
	// +required
	Location string `json:"location"`
}

// RuntimeSubstratePolicy contains the Substrate policy shared by all runtime variants.
//
// +kubebuilder:validation:XValidation:rule="self.workerPoolRef.name.size() > 0",message="workerPoolRef name must not be empty"
type RuntimeSubstratePolicy struct {
	// WorkerPoolRef references a WorkerPool in the resource's namespace.
	// +required
	WorkerPoolRef corev1.LocalObjectReference `json:"workerPoolRef"`

	// SnapshotPolicy configures runtime snapshot storage.
	// +required
	SnapshotPolicy RuntimeSnapshotPolicy `json:"snapshotPolicy"`

	// WakeupProbeTimeoutSeconds is how long Substrate waits for the runtime's
	// readiness probe to succeed after it starts. Defaults to 30 when unset.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	// +optional
	WakeupProbeTimeoutSeconds *int32 `json:"wakeupProbeTimeoutSeconds,omitempty"`
}
