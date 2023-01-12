package eventrule

import (
	"context"
	"fmt"
	"strings"

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

	queueName := eventrule.Name
	eventBusName := "default"
	eventRuleName := eventrule.Name
	eventPattern := "{\"detail\": {\"Publisher\": [\"kube-event-bridge\"]}}"
	eventsAccessor := events.NewEventsAccessor()

	res, err := c.sqsClient.CreateQueue(&sqs.CreateQueueInput{
		QueueName: &queueName,
	})

	if err != nil {
		return reconcile.Result{}, err
	}

	queueurl := res.QueueUrl

	_, err = eventsAccessor.CreateEventRule(&eventRuleName, &eventBusName, &eventPattern)

	if err != nil {
		return reconcile.Result{}, err
	}

	targetarn := constructArnFromQueueUrl(queueurl)
	err = eventsAccessor.PutTargets(&eventRuleName, &eventBusName, targetarn)

	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func constructArnFromQueueUrl(queueurl *string) *string {
	url := *queueurl
	urlarr := strings.Split(url, "/")
	queuename := urlarr[4]
	accountid := urlarr[3]
	region := strings.Split(urlarr[2], ".")[1]
	queuearn := fmt.Sprintf("%s%s%s%s%s%s", "arn:aws:sqs:", region, ":", accountid, ":", queuename)
	return &queuearn
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1alpha1.EventRule{}).
		Complete(NewController(m.GetClient()))
}
