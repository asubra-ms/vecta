// The Mutation Logic: Generates the RFC 6902 JSON Patch to modify the customer's pod on the fly
// This is the "logic-core" for the vecta wrap cmd's promise. It intercepts any pod creation in
// the vecta managed namespace and injects the vecta-sentry container into it.
// We implement this as a Sidecar automator

package webhook

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func CreatePatch(pod *corev1.Pod, tenantID string) ([]byte, error) {
	var patch []patchOperation

	// 1. Define the Vecta-Sentry Sidecar Container
	sentryContainer := corev1.Container{
		Name:  "vecta-sentry",
		Image: "localhost:5000/vecta-sentry:latest", // Pulls from local registry
		Ports: []corev1.ContainerPort{{ContainerPort: 8000}},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "spiffe-workload-api", MountPath: "/run/spire/sockets", ReadOnly: true},
			{Name: "enclave-config", MountPath: "/etc/vecta"},
		},
		Env: []corev1.EnvVar{
			{Name: "TENANT_ID", Value: tenantID},
			{Name: "Warden_MODE", Value: "audit"}, // Default to audit for discovery
		},
	}

	// 2. Add the container to the Pod spec
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/containers/-",
		Value: sentryContainer,
	})

	// 3. Add the SPIRE and ConfigMap Volumes
	volumes := []corev1.Volume{
		{
			Name: "spiffe-workload-api",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/run/spire/sockets"},
			},
		},
		{
			Name: "enclave-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "enclave-manifest"},
				},
			},
		},
	}

	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/volumes",
		Value: volumes,
	})

	return json.Marshal(patch)
}
