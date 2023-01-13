package eventrule

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/ellistarn/kube-event-bridge/pkg/accessor/events"
	"github.com/ellistarn/kube-event-bridge/pkg/apis/v1alpha1"
	"github.com/samber/lo"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		sqsClient:  sqs.New(lo.Must(session.NewSession())),
		kubeClient: kubeClient,
	}
}

type Controller struct {
	sqsClient  *sqs.SQS
	kubeClient client.Client
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	fmt.Println("Reconcile invoked in eventRule controller")

	eventrule := &v1alpha1.EventRule{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, eventrule); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	eventBusName := "default"
	eventRuleName := eventrule.Name
	eventPattern := "{\"source\": [{  \"prefix\": \"kube-event-bridge\"}]}"
	eventsAccessor := events.NewEventsAccessor()
	if _, err := eventsAccessor.CreateEventRule(&eventRuleName, &eventBusName, &eventPattern); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1alpha1.EventRule{}).
		Complete(NewController(m.GetClient()))
}
