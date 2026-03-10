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

// CreatePatch generates the JSON patch for the Mutating Admission Webhook.
func CreatePatch(pod *corev1.Pod, tenantID string) ([]byte, error) {
	var patch []patchOperation

	// 1. Define the Vecta-Sentry Sidecar Container
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
			// PORTABILITY FIX: Dynamically discover Host IP for Orchestrator communication
			{
				Name: "VECTA_HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
		},
	}

	// 2. Inject the Warden Sidecar
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/containers/-",
		Value: sentryContainer,
	})

	// 3. Define the Production-Grade Volumes
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

// PatchPodForSovereignty is required by server.go for direct orchestration.
func PatchPodForSovereignty(pod *corev1.Pod) {
	tenantID := pod.Labels["tenant"]

	sentryContainer := corev1.Container{
		Name:  "sentry-warden",
		Image: "localhost:5000/vecta-sentry:latest",
		VolumeMounts: []corev1.VolumeMount{
			{Name: "spiffe-workload-api", MountPath: "/run/spire/sockets", ReadOnly: true},
			{Name: "policy-vol", MountPath: "/var/vecta/policy", ReadOnly: true},
			{Name: "lib-vol", MountPath: "/var/vecta/lib"},
		},
		Env: []corev1.EnvVar{
			{Name: "TENANT_ID", Value: tenantID},
			// PORTABILITY FIX: Map Host IP dynamically
			{
				Name: "VECTA_HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
			// Warden code should use http://$VECTA_HOST_IP:8000
		},
	}

	pod.Spec.Containers = append(pod.Spec.Containers, sentryContainer)

	pod.Spec.Volumes = append(pod.Spec.Volumes, []corev1.Volume{
		{
			Name: "spiffe-workload-api",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/run/spire/sockets",
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
	}...)
}

func ptrBool(b bool) *bool {
	return &b
}
