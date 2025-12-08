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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cleanupv1alpha1 "github.com/cloud-club/08th-k8s-operator/monitoring-been/api/v1alpha1"
)

// CleanupPolicyReconciler reconciles a CleanupPolicy object
type CleanupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Cron scheduler for managing cleanup schedules
	cronScheduler *cron.Cron
	cronEntries   map[string]cron.EntryID
}

// +kubebuilder:rbac:groups=cleanup.cloudclub.com,resources=cleanuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cleanup.cloudclub.com,resources=cleanuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cleanup.cloudclub.com,resources=cleanuppolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *CleanupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the CleanupPolicy instance
	policy := &cleanupv1alpha1.CleanupPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("CleanupPolicy resource not found. Ignoring since object must be deleted.")
			// Remove cron job if exists
			if r.cronScheduler != nil && r.cronEntries != nil {
				if entryID, exists := r.cronEntries[req.NamespacedName.String()]; exists {
					r.cronScheduler.Remove(entryID)
					delete(r.cronEntries, req.NamespacedName.String())
					log.Info("Removed cron schedule for deleted policy", "policy", req.NamespacedName)
				}
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get CleanupPolicy")
		return ctrl.Result{}, err
	}

	// Initialize cron scheduler if not exists
	if r.cronScheduler == nil {
		r.cronScheduler = cron.New()
		r.cronScheduler.Start()
		r.cronEntries = make(map[string]cron.EntryID)
	}

	// Setup or update cron schedule
	schedule := policy.Spec.Schedule
	if schedule == "" {
		schedule = "0 6 * * *" // Default: daily at 6 AM
	}

	policyKey := req.NamespacedName.String()

	// Remove existing schedule if exists
	if entryID, exists := r.cronEntries[policyKey]; exists {
		r.cronScheduler.Remove(entryID)
		log.Info("Removed old cron schedule", "policy", policyKey)
	}

	// Add new cron schedule
	entryID, err := r.cronScheduler.AddFunc(schedule, func() {
		log.Info("Executing scheduled cleanup", "policy", policyKey, "schedule", schedule)

		// Create a new context for the cleanup job
		cleanupCtx := context.Background()

		// Fetch the latest policy
		latestPolicy := &cleanupv1alpha1.CleanupPolicy{}
		if err := r.Get(cleanupCtx, req.NamespacedName, latestPolicy); err != nil {
			log.Error(err, "Failed to fetch policy during scheduled cleanup")
			return
		}

		// Execute cleanup
		if err := r.executeCleanup(cleanupCtx, latestPolicy, log); err != nil {
			log.Error(err, "Failed to execute cleanup")
			r.updateCondition(cleanupCtx, latestPolicy, "CleanupFailed", metav1.ConditionFalse, "CleanupError", err.Error())
		} else {
			log.Info("Cleanup completed successfully")
			r.updateCondition(cleanupCtx, latestPolicy, "CleanupSucceeded", metav1.ConditionTrue, "CleanupCompleted", "Cleanup executed successfully")
		}
	})

	if err != nil {
		log.Error(err, "Failed to schedule cleanup cron job")
		return ctrl.Result{}, err
	}

	r.cronEntries[policyKey] = entryID
	log.Info("Scheduled cleanup cron job", "policy", policyKey, "schedule", schedule)

	// Update status with next execution time
	nextRun := r.cronScheduler.Entry(entryID).Next
	policy.Status.NextExecutionTime = &metav1.Time{Time: nextRun}

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update policy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// executeCleanup performs the actual cleanup operation
func (r *CleanupPolicyReconciler) executeCleanup(ctx context.Context, policy *cleanupv1alpha1.CleanupPolicy, log logr.Logger) error {
	log.Info("Starting cleanup execution", "policy", policy.Name)

	var pendingResources []cleanupv1alpha1.PendingResource
	totalCleaned := 0

	// Clean up Pods if enabled
	if policy.Spec.PodPolicies != nil && policy.Spec.PodPolicies.Enabled {
		cleaned, pending, err := r.cleanupPods(ctx, policy, log)
		if err != nil {
			log.Error(err, "Failed to cleanup pods")
		} else {
			totalCleaned += cleaned
			pendingResources = append(pendingResources, pending...)
			log.Info("Pod cleanup completed", "cleaned", cleaned, "pending", len(pending))
		}
	}

	// Clean up PersistentVolumes if enabled
	if policy.Spec.PVPolicies != nil && policy.Spec.PVPolicies.Enabled {
		cleaned, pending, err := r.cleanupPVs(ctx, policy, log)
		if err != nil {
			log.Error(err, "Failed to cleanup PVs")
		} else {
			totalCleaned += cleaned
			pendingResources = append(pendingResources, pending...)
			log.Info("PV cleanup completed", "cleaned", cleaned, "pending", len(pending))
		}
	}

	// Update status
	now := metav1.Now()
	policy.Status.LastExecutionTime = &now
	policy.Status.CleanedUp += totalCleaned
	policy.Status.PendingApproval = pendingResources

	if err := r.Status().Update(ctx, policy); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.Info("Cleanup execution completed", "totalCleaned", totalCleaned, "pendingApproval", len(pendingResources))
	return nil
}

// cleanupPods handles Pod cleanup logic
func (r *CleanupPolicyReconciler) cleanupPods(ctx context.Context, policy *cleanupv1alpha1.CleanupPolicy, log logr.Logger) (int, []cleanupv1alpha1.PendingResource, error) {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList); err != nil {
		return 0, nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var pendingResources []cleanupv1alpha1.PendingResource
	cleaned := 0

	for _, pod := range podList.Items {
		// Skip if namespace is excluded
		if r.isNamespaceExcluded(pod.Namespace, policy) {
			continue
		}

		// Check if namespace is included (if specified)
		if !r.isNamespaceIncluded(pod.Namespace, policy) {
			continue
		}

		var shouldCleanup bool
		var reason string

		// Check failed pods
		if policy.Spec.PodPolicies.FailedPods != nil && policy.Spec.PodPolicies.FailedPods.Enabled {
			if shouldDelete, deleteReason := r.shouldDeleteFailedPod(&pod, policy.Spec.PodPolicies.FailedPods); shouldDelete {
				shouldCleanup = true
				reason = deleteReason
			}
		}

		// Note: Idle pod checking requires metrics-server integration
		// This will be implemented in a future iteration when Prometheus is integrated

		if shouldCleanup {
			if policy.Spec.RequireApproval {
				// Add to pending approval list
				pendingResources = append(pendingResources, cleanupv1alpha1.PendingResource{
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Reason:    reason,
					MarkedAt:  metav1.Now(),
				})
				log.Info("Pod marked for approval", "namespace", pod.Namespace, "name", pod.Name, "reason", reason)
			} else if !policy.Spec.DryRun {
				// Actually delete the pod
				if err := r.Delete(ctx, &pod); err != nil {
					log.Error(err, "Failed to delete pod", "namespace", pod.Namespace, "name", pod.Name)
				} else {
					cleaned++
					log.Info("Deleted pod", "namespace", pod.Namespace, "name", pod.Name, "reason", reason)
				}
			} else {
				// Dry run mode - just log
				log.Info("[DRY-RUN] Would delete pod", "namespace", pod.Namespace, "name", pod.Name, "reason", reason)
			}
		}
	}

	return cleaned, pendingResources, nil
}

// shouldDeleteFailedPod checks if a failed pod should be deleted
func (r *CleanupPolicyReconciler) shouldDeleteFailedPod(pod *corev1.Pod, policy *cleanupv1alpha1.FailedPodPolicy) (bool, string) {
	// Check if pod is in a failed state
	podStatus := string(pod.Status.Phase)
	isFailedState := false

	for _, state := range policy.States {
		if podStatus == state {
			isFailedState = true
			break
		}
	}

	// Also check container statuses for CrashLoopBackOff, ImagePullBackOff
	if !isFailedState {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				waitingReason := containerStatus.State.Waiting.Reason
				for _, state := range policy.States {
					if waitingReason == state {
						isFailedState = true
						podStatus = waitingReason
						break
					}
				}
			}
		}
	}

	if !isFailedState {
		return false, ""
	}

	// Parse minAge duration
	minAge := policy.MinAge
	if minAge == "" {
		minAge = "3h"
	}

	duration, err := parseDuration(minAge)
	if err != nil {
		return false, ""
	}

	// Check if pod is old enough
	age := time.Since(pod.CreationTimestamp.Time)
	if age < duration {
		return false, ""
	}

	return true, fmt.Sprintf("Pod in %s state for %s (threshold: %s)", podStatus, age.Round(time.Minute), minAge)
}

// cleanupPVs handles PersistentVolume cleanup logic
func (r *CleanupPolicyReconciler) cleanupPVs(ctx context.Context, policy *cleanupv1alpha1.CleanupPolicy, log logr.Logger) (int, []cleanupv1alpha1.PendingResource, error) {
	pvList := &corev1.PersistentVolumeList{}
	if err := r.List(ctx, pvList); err != nil {
		return 0, nil, fmt.Errorf("failed to list PVs: %w", err)
	}

	var pendingResources []cleanupv1alpha1.PendingResource
	cleaned := 0

	minAge := policy.Spec.PVPolicies.MinAge
	if minAge == "" {
		minAge = "14d"
	}

	duration, err := parseDuration(minAge)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid minAge duration: %w", err)
	}

	for _, pv := range pvList.Items {
		// Check if PV is in target state
		isTargetState := false
		pvPhase := string(pv.Status.Phase)

		for _, state := range policy.Spec.PVPolicies.States {
			if pvPhase == state {
				isTargetState = true
				break
			}
		}

		if !isTargetState {
			continue
		}

		// Check age
		age := time.Since(pv.CreationTimestamp.Time)
		if age < duration {
			continue
		}

		reason := fmt.Sprintf("PV in %s state for %s (threshold: %s)", pvPhase, age.Round(time.Hour*24), minAge)

		if policy.Spec.RequireApproval {
			// Add to pending approval list
			pendingResources = append(pendingResources, cleanupv1alpha1.PendingResource{
				Kind:     "PersistentVolume",
				Name:     pv.Name,
				Reason:   reason,
				MarkedAt: metav1.Now(),
			})
			log.Info("PV marked for approval", "name", pv.Name, "reason", reason)
		} else if !policy.Spec.DryRun {
			// Actually delete the PV
			if err := r.Delete(ctx, &pv); err != nil {
				log.Error(err, "Failed to delete PV", "name", pv.Name)
			} else {
				cleaned++
				log.Info("Deleted PV", "name", pv.Name, "reason", reason)
			}
		} else {
			// Dry run mode - just log
			log.Info("[DRY-RUN] Would delete PV", "name", pv.Name, "reason", reason)
		}
	}

	return cleaned, pendingResources, nil
}

// isNamespaceExcluded checks if namespace should be excluded
func (r *CleanupPolicyReconciler) isNamespaceExcluded(namespace string, policy *cleanupv1alpha1.CleanupPolicy) bool {
	for _, excluded := range policy.Spec.ExcludeNamespaces {
		if namespace == excluded {
			return true
		}
	}
	return false
}

// isNamespaceIncluded checks if namespace is in include list (or if include list is empty)
func (r *CleanupPolicyReconciler) isNamespaceIncluded(namespace string, policy *cleanupv1alpha1.CleanupPolicy) bool {
	// If no include list specified, include all (except excluded)
	if len(policy.Spec.IncludeNamespaces) == 0 {
		return true
	}

	for _, included := range policy.Spec.IncludeNamespaces {
		if namespace == included {
			return true
		}
	}
	return false
}

// updateCondition updates the policy condition
func (r *CleanupPolicyReconciler) updateCondition(ctx context.Context, policy *cleanupv1alpha1.CleanupPolicy, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&policy.Status.Conditions, condition)
	r.Status().Update(ctx, policy)
}

// parseDuration parses duration strings like "3h", "14d", "30d"
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1:]
	value := s[:len(s)-1]

	var multiplier time.Duration
	switch unit {
	case "h":
		multiplier = time.Hour
	case "d":
		multiplier = time.Hour * 24
	default:
		return time.ParseDuration(s)
	}

	var num int
	_, err := fmt.Sscanf(value, "%d", &num)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}

	return time.Duration(num) * multiplier, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CleanupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cleanupv1alpha1.CleanupPolicy{}).
		Named("cleanuppolicy").
		Complete(r)
}
