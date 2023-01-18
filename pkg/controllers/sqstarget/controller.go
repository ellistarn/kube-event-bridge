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

var Finalizer = fmt.Sprintf("sqstarget.%s", v1alpha1.Group)

type Controller struct {
	kubeClient        client.Client
	sqsClient         *sqs.SQS
	eventBridgeClient *eventbridge.EventBridge
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	sqsTarget := &v1alpha1.SQSTarget{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, sqsTarget); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if err := lo.Ternary(sqsTarget.DeletionTimestamp.IsZero(), c.reconcile, c.finalize)(ctx, sqsTarget); err != nil {
		return reconcile.Result{}, err
	}
	if err := c.kubeClient.Status().Update(ctx, sqsTarget.DeepCopy()); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if err := c.kubeClient.Update(ctx, sqsTarget.DeepCopy()); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) reconcile(ctx context.Context, sqsTarget *v1alpha1.SQSTarget) error {
	// Create Queue
	createQueueOutput, err := c.sqsClient.CreateQueueWithContext(ctx, &sqs.CreateQueueInput{QueueName: aws.String(sqsTarget.Name)})
	if err != nil {
		return fmt.Errorf("creating queue, %w", err)
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
		return fmt.Errorf("creating event rule, %w", err)
	}
	// Update Status
	sqsTarget.Status.QueueURL = *createQueueOutput.QueueUrl
	controllerutil.AddFinalizer(sqsTarget, Finalizer)
	return nil
}

func (c *Controller) finalize(ctx context.Context, sqsTarget *v1alpha1.SQSTarget) error {
	if _, err := c.sqsClient.DeleteQueueWithContext(ctx, &sqs.DeleteQueueInput{
		QueueUrl: lo.ToPtr(sqsTarget.Status.QueueURL),
	}); err != nil && !strings.Contains(err.Error(), "AWS.SimpleQueueService.NonExistentQueue") {
		return fmt.Errorf("deleting queue, %w", err)
	}
	if _, err := c.eventBridgeClient.RemoveTargetsWithContext(ctx, &eventbridge.RemoveTargetsInput{
		EventBusName: aws.String("default"),
		Rule:         aws.String(sqsTarget.Spec.EventRule),
		Ids:          aws.StringSlice([]string{string(sqsTarget.UID)}),
	}); err != nil && !strings.Contains(err.Error(), "ResourceNotFoundException") {
		return fmt.Errorf("removing event rule targets, %w", err)
	}
	controllerutil.RemoveFinalizer(sqsTarget, Finalizer)
	return nil
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
