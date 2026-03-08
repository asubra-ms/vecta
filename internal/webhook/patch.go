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
			{Name: "WARDEN_MODE", Value: "audit"}, // Default to audit for discovery
		},
	}

	// 2. Add the container to the Pod spec
	// Using /spec/containers/- to append to the existing container list
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/containers/-",
		Value: sentryContainer,
	})

	// 3. Define the SPIRE and ConfigMap Volumes
	// We use the SPIFFE CSI Driver for hardened identity delivery
	newVolumes := []corev1.Volume{
		{
			Name: "spiffe-workload-api",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "csi.spiffe.io",
					ReadOnly: ptrBool(true),
				},
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

	// 4. Add the volumes to the Pod spec
	// Note: If the pod has no volumes, we create the list. If it has some, we append.
	// For this stable fabric, we append to ensure we don't overwrite agent volumes.
	for _, vol := range newVolumes {
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  "/spec/volumes/-",
			Value: vol,
		})
	}

	return json.Marshal(patch)
}

// Helper for CSI ReadOnly pointer
func ptrBool(b bool) *bool {
	return &b
}
