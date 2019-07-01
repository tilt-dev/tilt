package k8s

import (
	v1 "k8s.io/api/core/v1"
)

// A Kubernetes event and the entity to which it applies
type EventWithEntity struct {
	Event  *v1.Event
	Entity K8sEntity
}

func NewEventWithEntity(evt *v1.Event, entity K8sEntity) EventWithEntity {
	return EventWithEntity{
		Event:  evt,
		Entity: entity,
	}
}
