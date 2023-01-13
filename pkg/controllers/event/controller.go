package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewController(kubeClient client.Client) *Controller {
	sess := lo.Must(session.NewSession())
	return &Controller{
		kubeClient:        kubeClient,
		eventBridgeClient: eventbridge.New(sess),
	}
}

type Controller struct {
	kubeClient        client.Client
	eventBridgeClient *eventbridge.EventBridge
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Read event
	event := &v1.Event{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, event); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	// Publish to event bridge
	if _, err := c.eventBridgeClient.PutEvents(&eventbridge.PutEventsInput{
		Entries: []*eventbridge.PutEventsRequestEntry{{
			Detail:       aws.String(string(lo.Must(json.Marshal((event))))),
			DetailType:   aws.String("test"),
			EventBusName: aws.String("default"),
			Resources:    []*string{},
			Source:       aws.String("kube-event-bridge"),
		}},
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("posting events, %w", err)
	}
	return reconcile.Result{}, nil
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1.Event{}).
		Complete(NewController(m.GetClient()))
}
