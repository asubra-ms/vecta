// The Mutation Logic: Generates the RFC 6902 JSON Patch to modify the customer's pod on the fly.
// Updated for Vecta V3: Uses HostPath mounts for policy enforcement and intent discovery.

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
	// Updated to use the /var/vecta production paths
	sentryContainer := corev1.Container{
		Name:  "sentry-warden",
		Image: "localhost:5000/vecta-sentry:latest",
		Ports: []corev1.ContainerPort{{ContainerPort: 8000}},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "spiffe-workload-api", MountPath: "/run/spire/sockets", ReadOnly: true},
			{Name: "policy-vol", MountPath: "/var/vecta/policy", ReadOnly: true},
			{Name: "lib-vol", MountPath: "/var/vecta/lib"},
		},
		Env: []corev1.EnvVar{
			{Name: "TENANT_ID", Value: tenantID},
			// The Warden reads VECTA_AUDIT_TIME or the policy.yaml file directly
		},
	}

	// 2. Inject the Warden Sidecar
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/containers/-",
		Value: sentryContainer,
	})

	// 3. Define the Production-Grade Volumes
	// - SPIFFE CSI: For Identity
	// - HostPath (Policy): For Read-Only Governance
	// - HostPath (Lib): For Persisting Discovered Intent
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
			Name: "policy-vol",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/vecta/policy",
				},
			},
		},
		{
			Name: "lib-vol",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/vecta/lib",
				},
			},
		},
	}

	// 4. Inject Volumes into the Pod spec
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
