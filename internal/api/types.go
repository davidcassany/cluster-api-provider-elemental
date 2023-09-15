package api

import (
	"fmt"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HostCreateRequest struct {
	Name        string            `json:"name,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func (h *HostCreateRequest) toElementalHost(namespace string) infrastructurev1beta1.ElementalHost {
	return infrastructurev1beta1.ElementalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name,
			Namespace:   namespace,
			Labels:      h.Labels,
			Annotations: h.Annotations,
		},
	}
}

type HostPatchRequest struct {
	Annotations  map[string]string `json:"annotations,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Bootstrapped *bool             `json:"bootstrapped,omitempty"`
	Installed    *bool             `json:"installed,omitempty"`
}

func (h *HostPatchRequest) fromElementalHost(elementalHost *infrastructurev1beta1.ElementalHost) {
	elementalHost.Annotations = h.Annotations
	elementalHost.Labels = h.Labels
	if h.Installed != nil {
		elementalHost.Status.Installed = *h.Installed
	}
	if h.Bootstrapped != nil {
		elementalHost.Status.Bootstrapped = *h.Bootstrapped
	}
}

type HostResponse struct {
	Name           string            `json:"name,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	BootstrapReady bool              `json:"bootstrapReady,omitempty"`
	Bootstrapped   bool              `json:"bootstrapped,omitempty"`
	Installed      bool              `json:"installed,omitempty"`
}

func (h *HostResponse) fromElementalHost(elementalHost infrastructurev1beta1.ElementalHost) {
	h.Name = elementalHost.Name
	h.Annotations = elementalHost.Annotations
	h.Labels = elementalHost.Labels
	h.BootstrapReady = elementalHost.Spec.BootstrapSecret != nil
	h.Bootstrapped = elementalHost.Status.Bootstrapped
	h.Installed = elementalHost.Status.Installed
}

type RegistrationResponse struct {
	// MachineLabels are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	MachineLabels map[string]string `json:"machineLabels,omitempty"`
	// MachineAnnotations are labels propagated to each ElementalHost object linked to this registration.
	// +optional
	MachineAnnotations map[string]string `json:"machineAnnotations,omitempty"`
	// Config points to Elemental machine configuration.
	// +optional
	Config *infrastructurev1beta1.Config `json:"config,omitempty"`
}

func (r *RegistrationResponse) fromElementalMachineRegistration(elementalRegistration infrastructurev1beta1.ElementalMachineRegistration) {
	r.MachineLabels = elementalRegistration.Spec.MachineLabels
	r.MachineAnnotations = elementalRegistration.Spec.MachineAnnotations
	r.Config = elementalRegistration.Spec.Config
}

type BootstrapResponse struct {
	Files    []BootstrapFile `json:"writeFiles" yaml:"writeFiles"`
	Commands []string        `json:"runCmd" yaml:"runCmd"`
}

type BootstrapFile struct {
	Path        string `json:"path" yaml:"path"`
	Owner       string `json:"owner" yaml:"owner"`
	Permissions string `json:"permissions" yaml:"permissions"`
	Content     string `json:"content" yaml:"content"`
}

func (b *BootstrapResponse) fromSecret(secret *corev1.Secret) error {
	data := secret.Data["value"]
	if err := yaml.Unmarshal(data, b); err != nil {
		return fmt.Errorf("unmarshalling bootstrap secret value: %w", err)
	}
	return nil
}
