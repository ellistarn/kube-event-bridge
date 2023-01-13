package sqstarget

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/ellistarn/kube-event-bridge/pkg/apis/v1alpha1"
	"github.com/samber/lo"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewController(kubeClient client.Client) *Controller {
	sess := lo.Must(session.NewSession())
	return &Controller{
		sqsClient:         sqs.New(sess),
		kubeClient:        kubeClient,
		eventBridgeClient: eventbridge.New(sess),
	}
}

type Controller struct {
	sqsClient         *sqs.SQS
	kubeClient        client.Client
	eventBridgeClient *eventbridge.EventBridge
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	sqsTarget := &v1alpha1.SQSTarget{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, sqsTarget); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Create Queue
	createQueueOutput, err := c.sqsClient.CreateQueueWithContext(ctx, &sqs.CreateQueueInput{QueueName: &req.Name})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("creating queue, %w", err)
	}

	// Create SQS Target
	if _, err := c.eventBridgeClient.PutTargets(&eventbridge.PutTargetsInput{
		EventBusName: aws.String("default"),
		Rule:         aws.String(sqsTarget.Spec.EventRule),
		Targets: []*eventbridge.Target{{
			Id:  aws.String(string(sqsTarget.UID)),
			Arn: arnFromQueueUrl(createQueueOutput.QueueUrl),
		}},
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("creating event rule, %w", err)
	}

	// Update Status
	sqsTarget.Status.QueueURL = *createQueueOutput.QueueUrl
	if err := c.kubeClient.Status().Update(ctx, sqsTarget); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	return reconcile.Result{}, nil
}

func arnFromQueueUrl(queueurl *string) *string {
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
		For(&v1alpha1.SQSTarget{}).
		Complete(NewController(m.GetClient()))
}
