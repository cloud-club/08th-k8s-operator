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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	eighthv1alpha1 "github.com/cloud-club/08th-k8s-operator/api/v1alpha1"
)

// MyAppReconciler reconciles a MyApp object
type MyAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=eighth.cloudclub.com,resources=myapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=eighth.cloudclub.com,resources=myapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=eighth.cloudclub.com,resources=myapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the MyApp instance
	myApp := &eighthv1alpha1.MyApp{}
	if err := r.Get(ctx, req.NamespacedName, myApp); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch MyApp")
		return ctrl.Result{}, err
	}

	// Set default replicas if not specified
	replicas := int32(1)
	if myApp.Spec.Replicas != nil {
		replicas = *myApp.Spec.Replicas
	}

	// Validate username
	if myApp.Spec.Username == "" {
		log.Info("username is required, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Reconcile ConfigMap
	if err := r.reconcileConfigMap(ctx, myApp); err != nil {
		log.Error(err, "unable to reconcile ConfigMap")
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	if err := r.reconcileDeployment(ctx, myApp, replicas); err != nil {
		log.Error(err, "unable to reconcile Deployment")
		return ctrl.Result{}, err
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, myApp); err != nil {
		log.Error(err, "unable to reconcile Service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileConfigMap creates or updates the ConfigMap for nginx configuration
func (r *MyAppReconciler) reconcileConfigMap(ctx context.Context, myApp *eighthv1alpha1.MyApp) error {
	log := logf.FromContext(ctx)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-nginx-config", myApp.Name),
			Namespace: myApp.Namespace,
		},
	}

	// Create or update the ConfigMap
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		// Generate nginx HTML template with username (Pod IP will be injected at runtime)
		htmlTemplate := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
    <title>Welcome</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            text-align: center;
            padding: 50px;
            background-color: #f0f0f0;
        }
        h1 {
            color: #333;
        }
        .pod-info {
            margin-top: 20px;
            padding: 15px;
            background-color: #fff;
            border-radius: 5px;
            display: inline-block;
        }
        .pod-ip {
            color: #0066cc;
            font-weight: bold;
        }
    </style>
</head>
<body>
    <h1>안녕하세요! %s 클둥이님!</h1>
    <div class="pod-info">
        <p>현재 접속된 Pod IP: <span class="pod-ip">${POD_IP}</span></p>
    </div>
</body>
</html>`, myApp.Spec.Username)

		configMap.Data = map[string]string{
			"index.html.template": htmlTemplate,
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(myApp, configMap, r.Scheme); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("ConfigMap reconciled", "operation", op, "name", configMap.Name)
	}

	return nil
}

// reconcileDeployment creates or updates the Deployment for nginx
func (r *MyAppReconciler) reconcileDeployment(ctx context.Context, myApp *eighthv1alpha1.MyApp, replicas int32) error {
	log := logf.FromContext(ctx)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-nginx", myApp.Name),
			Namespace: myApp.Namespace,
		},
	}

	// Create or update the Deployment
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app":   "nginx",
				"myapp": myApp.Name,
			},
		}

		deployment.Spec.Template.ObjectMeta.Labels = map[string]string{
			"app":   "nginx",
			"myapp": myApp.Name,
		}

		// Init container to generate HTML with Pod IP
		deployment.Spec.Template.Spec.InitContainers = []corev1.Container{
			{
				Name:  "html-generator",
				Image: "busybox:1.36",
				Env: []corev1.EnvVar{
					{
						Name: "POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				Command: []string{"/bin/sh"},
				Args: []string{
					"-c",
					"sed \"s|\\${POD_IP}|$POD_IP|g\" /templates/index.html.template > /html/index.html",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "nginx-config",
						MountPath: "/templates",
						ReadOnly:  true,
					},
					{
						Name:      "nginx-html",
						MountPath: "/html",
					},
				},
			},
		}

		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  "nginx",
				Image: "nginx:1.25-alpine",
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 80,
						Name:          "http",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "nginx-html",
						MountPath: "/usr/share/nginx/html",
					},
				},
			},
		}

		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "nginx-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: fmt.Sprintf("%s-nginx-config", myApp.Name),
						},
					},
				},
			},
			{
				Name: "nginx-html",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(myApp, deployment, r.Scheme); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("Deployment reconciled", "operation", op, "name", deployment.Name)
	}

	return nil
}

// reconcileService creates or updates the Service for nginx
func (r *MyAppReconciler) reconcileService(ctx context.Context, myApp *eighthv1alpha1.MyApp) error {
	log := logf.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-nginx", myApp.Name),
			Namespace: myApp.Namespace,
		},
	}

	// Create or update the Service
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Selector = map[string]string{
			"app":   "nginx",
			"myapp": myApp.Name,
		}

		service.Spec.Ports = []corev1.ServicePort{
			{
				Port:       80,
				TargetPort: intstr.FromInt32(80),
				Protocol:   corev1.ProtocolTCP,
				Name:       "http",
			},
		}

		service.Spec.Type = corev1.ServiceTypeClusterIP

		// Set owner reference
		if err := controllerutil.SetControllerReference(myApp, service, r.Scheme); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("Service reconciled", "operation", op, "name", service.Name)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&eighthv1alpha1.MyApp{}).
		Named("myapp").
		Complete(r)
}
