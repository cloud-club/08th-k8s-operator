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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// resourcesCleanedTotal tracks total number of resources cleaned
	resourcesCleanedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cleanup_operator_resources_cleaned_total",
			Help: "Total number of resources cleaned by the operator",
		},
		[]string{"kind", "namespace"},
	)

	// cleanupExecutionDuration tracks cleanup execution duration
	cleanupExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cleanup_operator_execution_duration_seconds",
			Help:    "Duration of cleanup execution in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"policy"},
	)

	// cleanupExecutionTotal tracks total cleanup executions
	cleanupExecutionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cleanup_operator_execution_total",
			Help: "Total number of cleanup executions",
		},
		[]string{"policy", "status"},
	)

	// pendingApprovalResources tracks resources pending approval
	pendingApprovalResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cleanup_operator_pending_approval_resources",
			Help: "Number of resources pending approval",
		},
		[]string{"policy", "kind"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		resourcesCleanedTotal,
		cleanupExecutionDuration,
		cleanupExecutionTotal,
		pendingApprovalResources,
	)
}

// RecordResourceCleaned records a cleaned resource metric
func RecordResourceCleaned(kind, namespace string) {
	resourcesCleanedTotal.WithLabelValues(kind, namespace).Inc()
}

// RecordCleanupExecution records a cleanup execution metric
func RecordCleanupExecution(policy, status string, duration float64) {
	cleanupExecutionTotal.WithLabelValues(policy, status).Inc()
	cleanupExecutionDuration.WithLabelValues(policy).Observe(duration)
}

// UpdatePendingApprovalMetrics updates pending approval gauge metrics
func UpdatePendingApprovalMetrics(policy string, podCount, pvCount int) {
	pendingApprovalResources.WithLabelValues(policy, "Pod").Set(float64(podCount))
	pendingApprovalResources.WithLabelValues(policy, "PersistentVolume").Set(float64(pvCount))
}
