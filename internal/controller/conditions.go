package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
)

// setCondition updates or adds a condition on the GPUCluster status.
func setCondition(gc *v1alpha1.GPUCluster, condition metav1.Condition) {
	for i, existing := range gc.Status.Conditions {
		if existing.Type == condition.Type {
			if existing.Status != condition.Status {
				condition.LastTransitionTime = metav1.Now()
			} else {
				condition.LastTransitionTime = existing.LastTransitionTime
			}
			gc.Status.Conditions[i] = condition
			return
		}
	}
	if condition.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = metav1.Now()
	}
	gc.Status.Conditions = append(gc.Status.Conditions, condition)
}

// getCondition returns the condition with the given type, or nil if not found.
func getCondition(gc *v1alpha1.GPUCluster, condType string) *metav1.Condition {
	for i := range gc.Status.Conditions {
		if gc.Status.Conditions[i].Type == condType {
			return &gc.Status.Conditions[i]
		}
	}
	return nil
}
