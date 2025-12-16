/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ccpcroomkrv1 "github.com/cloud-club/08th-k8s-operator/operators/jeonjik-operator/api/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// JeonjikReconciler reconciles a Jeonjik object
type JeonjikReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cc-pc-room.kr,resources=jeonjiks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cc-pc-room.kr,resources=jeonjiks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cc-pc-room.kr,resources=jeonjiks/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Jeonjik object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *JeonjikReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// TODO(user): your logic here
	log.Info("▶ Reconcile triggered!", "name", req.NamespacedName)

	var cr ccpcroomkrv1.Jeonjik
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	specTypes := map[string]struct {
		CPU    string
		Memory string
		OS     string
		GPU    string
	}{
		"slave-jiwon":  {"2c", "2m", "ubuntu-24", "0"},
		"seoksa-jiwon": {"3c", "3m", "ubuntu-24", "0"},
		"baksa-jiwon":  {"4c", "4m", "ubuntu-24", "0"},
		"h100-jjaegii": {"4c", "4m", "rocky-9", "4gpu"},
		"l40s-jjaegii": {"8c", "8m", "rocky-9", "8gpu"},
		"moon0-potato": {"4c", "4m", "ubuntu-24", "0"},
	}

	// check specType exists
	spec, exists := specTypes[cr.Spec.SpecType]
	if !exists {
		log.Info("Unknown specType", "specType", cr.Spec.SpecType)
		return ctrl.Result{}, nil
	}

	// update status
	cr.Status.CPU = spec.CPU
	cr.Status.Memory = spec.Memory
	cr.Status.OS = spec.OS
	cr.Status.GPU = spec.GPU
	cr.Status.Ready = true

	if err := r.Status().Update(ctx, &cr); err != nil {
		log.Error(err, "Failed to update Jeonjik status")
		return ctrl.Result{}, err
	}

	// create VirtualMachine in kubevirt-instance namespace if it doesn't exist
	// Once created, KubeVirt controller will manage it. We don't update it.
	vmNamespace := "kubevirt-instance"
	vmName := cr.Name

	var vm kubevirtv1.VirtualMachine
	err := r.Get(ctx, client.ObjectKey{Namespace: vmNamespace, Name: vmName}, &vm)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get VirtualMachine", "name", vmName, "namespace", vmNamespace)
			return ctrl.Result{}, err
		}
		// VirtualMachine doesn't exist, create it
		vm = r.buildVirtualMachine(cr, spec, vmName, vmNamespace)
		if err := r.Create(ctx, &vm); err != nil {
			log.Error(err, "Failed to create VirtualMachine", "name", vmName, "namespace", vmNamespace)
			return ctrl.Result{}, err
		}
		log.Info("VirtualMachine created", "name", vmName, "namespace", vmNamespace)
	} else {
		// VirtualMachine already exists, do nothing (KubeVirt controller manages it)
		log.Info("VirtualMachine already exists, skipping", "name", vmName, "namespace", vmNamespace)
	}

	log.Info("Jeonjik status updated", "name", cr.Name)

	return ctrl.Result{}, nil
}

// buildVirtualMachine creates a VirtualMachine resource based on Jeonjik spec
func (r *JeonjikReconciler) buildVirtualMachine(cr ccpcroomkrv1.Jeonjik, spec struct {
	CPU    string
	Memory string
	OS     string
	GPU    string
}, vmName, vmNamespace string) kubevirtv1.VirtualMachine {
	// Parse CPU (e.g., "2c" -> 2)
	cpuStr := strings.TrimSuffix(spec.CPU, "c")
	cpuCores, _ := strconv.ParseInt(cpuStr, 10, 32)

	// Parse Memory (e.g., "2m" -> 2Gi, assuming "m" means Gi)
	memoryStr := strings.TrimSuffix(spec.Memory, "m")
	memoryGi, _ := strconv.ParseInt(memoryStr, 10, 32)
	memoryQuantity := resource.MustParse(fmt.Sprintf("%dGi", memoryGi))

	// Build VirtualMachine spec
	vm := kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: vmNamespace,
			Labels: map[string]string{
				"app":        "jeonjik",
				"jeonjik-cr": cr.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cr.APIVersion,
					Kind:       cr.Kind,
					Name:       cr.Name,
					UID:        cr.UID,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: func() *kubevirtv1.VirtualMachineRunStrategy {
				strategy := kubevirtv1.RunStrategyAlways
				return &strategy
			}(),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "jeonjik",
						"jeonjik-cr": cr.Name,
					},
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						CPU: &kubevirtv1.CPU{
							Cores: uint32(cpuCores),
						},
						Memory: &kubevirtv1.Memory{
							Guest: &memoryQuantity,
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name: "containerdisk",
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: "containerdisk",
							VolumeSource: kubevirtv1.VolumeSource{
								ContainerDisk: &kubevirtv1.ContainerDiskSource{
									Image: r.getOSImage(spec.OS),
								},
							},
						},
					},
				},
			},
		},
	}

	// Add GPU if specified
	if spec.GPU != "0" {
		gpuCount, _ := strconv.ParseInt(strings.TrimSuffix(spec.GPU, "gpu"), 10, 32)
		if gpuCount > 0 {
			vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{
				{
					Name:       "gpu",
					DeviceName: "nvidia.com/gpu",
				},
			}
			vm.Spec.Template.Spec.Domain.Resources = kubevirtv1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse(fmt.Sprintf("%d", gpuCount)),
				},
			}
		}
	}

	return vm
}

// getOSImage returns the container image for the specified OS
func (r *JeonjikReconciler) getOSImage(os string) string {
	osImages := map[string]string{
		"ubuntu-24": "quay.io/kubevirt/fedora-cloud-container-disk-demo:latest",
		"rocky-9":   "quay.io/kubevirt/fedora-cloud-container-disk-demo:latest",
	}
	if image, ok := osImages[os]; ok {
		return image
	}
	// Default image
	return "quay.io/kubevirt/fedora-cloud-container-disk-demo:latest"
}

// SetupWithManager sets up the controller with the Manager.
func (r *JeonjikReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ccpcroomkrv1.Jeonjik{}).
		Named("jeonjik").
		Complete(r)
}
