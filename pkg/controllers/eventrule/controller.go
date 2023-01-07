package eventrule

import (
	"context"

	"github.com/ellistarn/kube-event-bridge/pkg/apis/v1alpha1"
	v1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		kubeClient:   kubeClient,
	}
}

type Controller struct {
	kubeClient   client.Client
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Read event
	event := &v1.Event{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, event); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	log.FromContext(ctx).Info("Got event", event)

	// Read all event rules
	eventRuleList := &v1alpha1.EventRuleList{}
	if err := c.kubeClient.List(ctx, eventRuleList); err != nil {
		return reconcile.Result{}, err
	}

	// Find matching rule and publish to event bus
	for _, eventRule := range eventRuleList.Items {
		if true {
			log.FromContext(ctx).Info("Found matching event", "eventRule", eventRule.Name)
		}
	}

	return reconcile.Result{}, nil
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1.Event{}).
		Complete(NewController(m.GetClient()))
}
