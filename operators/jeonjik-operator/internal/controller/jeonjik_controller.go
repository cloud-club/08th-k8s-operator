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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ccpcroomkrv1 "github.com/cloud-club/08th-k8s-operator/operators/jeonjik-operator/api/v1"
)

// JeonjikReconciler reconciles a Jeonjik object
type JeonjikReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cc-pc-room.kr,resources=jeonjiks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cc-pc-room.kr,resources=jeonjiks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cc-pc-room.kr,resources=jeonjiks/finalizers,verbs=update

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
		"h100-jjaegii": {"4c", "4m", "rocky-9", "4gpu"}, "l40s-jjaegii": {"8c", "8m", "rocky-9", "8gpu"},
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

	log.Info("Jeonjik status updated", "name", cr.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JeonjikReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ccpcroomkrv1.Jeonjik{}).
		Named("jeonjik").
		Complete(r)
}
