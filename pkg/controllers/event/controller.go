package event

import (
	"context"
	"encoding/json"

	"github.com/ellistarn/kube-event-bridge/pkg/accessor/events"
	v1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		kubeClient: kubeClient,
	}
}

type Controller struct {
	kubeClient client.Client
}

type EventDetail struct {
	Message   *string
	Publisher *string
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Read event
	event := &v1.Event{}
	eventBusName := "default"
	publisherName := "kube-event-bridge"

	if err := c.kubeClient.Get(ctx, req.NamespacedName, event); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	eventsAccessor := events.NewEventsAccessor()
	log.FromContext(ctx).Info("Got event",
		"reason", event.Reason,
		"message", event.Message,
		"type", event.Type,
	)

	eventDetail := EventDetail{
		Message:   &event.Message,
		Publisher: &publisherName,
	}
	msg, _ := json.Marshal(eventDetail)
	eventMessage := string(msg)
	log.FromContext(ctx).Info("printing event", "eventMessage= ", eventMessage)
	eventsAccessor.PutEvents(&eventBusName, &eventMessage, &event.Type, &event.Reason, &event.Name)

	return reconcile.Result{}, nil
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1.Event{}).
		Complete(NewController(m.GetClient()))
}
