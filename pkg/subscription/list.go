package subscription

import (
	"fmt"

	profilesv1 "github.com/weaveworks/profiles/api/v1alpha1"
)

// List returns a list of subscriptions
func (sm *Manager) List() ([]SubscriptionSummary, error) {
	var subscriptions profilesv1.ProfileSubscriptionList
	err := sm.kClient.List(sm.ctx, &subscriptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list profile subscriptions: %w", err)
	}
	var descriptions []SubscriptionSummary
	for _, sub := range subscriptions.Items {
		status := "Unknown"
		for _, cond := range sub.Status.Conditions {
			if cond.Type == "Ready" {
				status = string(sub.Status.Conditions[0].Status)
				break
			}
		}

		descriptions = append(descriptions, SubscriptionSummary{
			Name:      sub.Name,
			Namespace: sub.Namespace,
			Ready:     status,
		})
	}
	return descriptions, nil
}
