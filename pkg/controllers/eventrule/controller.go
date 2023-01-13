package eventrule

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/ellistarn/kube-event-bridge/pkg/accessor/events"
	"github.com/ellistarn/kube-event-bridge/pkg/apis/v1alpha1"
	"github.com/samber/lo"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewController(kubeClient client.Client) *Controller {
	sess := lo.Must(session.NewSession())
	return &Controller{
		kubeClient:        kubeClient,
		sqsClient:         sqs.New(sess),
		eventBridgeClient: eventbridge.New(sess),
	}
}

var Finalizer = fmt.Sprintf("eventrule.%s", v1alpha1.Group)

type Controller struct {
	kubeClient        client.Client
	sqsClient         *sqs.SQS
	eventBridgeClient *eventbridge.EventBridge
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	eventRule := &v1alpha1.EventRule{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, eventRule); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if err := lo.Ternary(eventRule.DeletionTimestamp.IsZero(), c.reconcile, c.finalize)(ctx, eventRule); err != nil {
		return reconcile.Result{}, err
	}
	if err := c.kubeClient.Update(ctx, eventRule.DeepCopy()); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) reconcile(ctx context.Context, eventRule *v1alpha1.EventRule) error {
	eventBusName := "default"
	eventRuleName := eventRule.Name
	eventPattern := "{\"source\":[{\"prefix\":\"kube-event-bridge\"}]}"
	eventsAccessor := events.NewEventsAccessor()
	if _, err := eventsAccessor.CreateEventRule(&eventRuleName, &eventBusName, &eventPattern); err != nil {
		return fmt.Errorf("creating event rule, %w", err)
	}
	controllerutil.AddFinalizer(eventRule, Finalizer)
	return nil
}

func (c *Controller) finalize(ctx context.Context, eventRule *v1alpha1.EventRule) error {
	if _, err := c.eventBridgeClient.DeleteRule(&eventbridge.DeleteRuleInput{
		EventBusName: aws.String("default"),
		Name:         aws.String(eventRule.Name),
	}); err != nil && !strings.Contains(err.Error(), "ResourceNotFoundException") {
		return fmt.Errorf("deleting event rule, %w", err)
	}

	controllerutil.RemoveFinalizer(eventRule, Finalizer)
	return nil
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1alpha1.EventRule{}).
		Complete(NewController(m.GetClient()))
}
