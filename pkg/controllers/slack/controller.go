package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/ellistarn/kube-event-bridge/pkg/apis/v1alpha1"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
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
	// Read SlackTarget
	slackTarget := &v1alpha1.SlackTarget{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, slackTarget); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: Create a SQSQueueTarget instead of hardcoding
	getQueueUrlOutput, err := c.sqsClient.GetQueueUrlWithContext(ctx, &sqs.GetQueueUrlInput{QueueName: lo.ToPtr(slackTarget.Name)})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting queue url, %w", err)
	}

	// Long poll messages from corresponding queue
	receiveMessagesOutput, err := c.sqsClient.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            getQueueUrlOutput.QueueUrl,
		MaxNumberOfMessages: aws.Int64(10),
		WaitTimeSeconds:     aws.Int64(20),
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("reading messages, %w", err)
	}

	// Publish to slack
	for _, message := range receiveMessagesOutput.Messages {
		body, err := Format(message)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("formatting message %s, %w", message, err)
		}
		_, err = http.Post(slackTarget.Spec.HTTPEndpoint, "application/json", body)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("posting to slack, %w", err)
		}
		if _, err := c.sqsClient.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      getQueueUrlOutput.QueueUrl,
			ReceiptHandle: message.ReceiptHandle,
		}); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting message, %w", err)
		}
	}
	// make sure the aws event rule is created
	return reconcile.Result{Requeue: true}, nil
}

func Format(message *sqs.Message) (io.Reader, error) {
	event := &v1.Event{}
	if err := json.Unmarshal([]byte(lo.FromPtr(message.Body)), event); err != nil {
		return nil, fmt.Errorf("unmarshalling json, %w", err)
	}
	b, err := json.Marshal(struct {
		Type           string `json:"type"`
		InvolvedObject string `json:"involvedObject"`
		Reason         string `json:"reason"`
		Message        string `json:"message"`
	}{
		Type:           formatType(event),
		InvolvedObject: formatInvolvedObject(event),
		Reason:         event.Reason,
		Message:        event.Message,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling json, %w", err)
	}
	return bytes.NewBuffer(b), nil
}

func formatInvolvedObject(event *v1.Event) string {
	if event.InvolvedObject.Namespace == "" {
		return fmt.Sprintf("%s/%s/%s", event.APIVersion, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
	}
	return fmt.Sprintf("%s/%s/%s/%s", event.APIVersion, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Namespace, event.InvolvedObject.Name)
}

func formatType(event *v1.Event) string {
	return lo.Switch[string, string](event.Type).
		Case(v1.EventTypeNormal, ":white_check_mark:").
		Case(v1.EventTypeWarning, ":warning:").
		Default(":x:")
}

func Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1alpha1.SlackTarget{}).
		Complete(NewController(m.GetClient()))
}
