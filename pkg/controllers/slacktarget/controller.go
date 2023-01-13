package slacktarget

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	// Create the underlying SQSTarget and retrieve QueueUrl
	sqsTarget, err := c.ensureSQSTarget(ctx, slackTarget)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("ensuring sqs target, %w", err)
	}
	if sqsTarget.Status.QueueURL == "" {
		return reconcile.Result{Requeue: true}, nil
	}

	// Long poll messages from the queue
	receiveMessagesOutput, err := c.sqsClient.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(sqsTarget.Status.QueueURL),
		MaxNumberOfMessages: aws.Int64(10),
		WaitTimeSeconds:     aws.Int64(20),
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("reading messages, %w", err)
	}

	// Publish to slack
	for _, message := range receiveMessagesOutput.Messages {
		log.FromContext(ctx).Info(fmt.Sprintf("got message, %s", *message.Body))
		body, err := Format(ctx, message)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("formatting message %s, %w", message, err)
		}
		_, err = http.Post(slackTarget.Spec.HTTPEndpoint, "application/json", body)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("posting to slack, %w", err)
		}
		if _, err := c.sqsClient.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(sqsTarget.Status.QueueURL),
			ReceiptHandle: message.ReceiptHandle,
		}); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting message, %w", err)
		}
	}
	// make sure the aws event rule is created
	return reconcile.Result{Requeue: true}, nil
}

func (c *Controller) ensureSQSTarget(ctx context.Context, slackTarget *v1alpha1.SlackTarget) (*v1alpha1.SQSTarget, error) {
	sqsTarget := &v1alpha1.SQSTarget{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: slackTarget.Name}, sqsTarget); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("getting sqs target, %w", err)
		}
		sqsTarget = &v1alpha1.SQSTarget{
			ObjectMeta: metav1.ObjectMeta{Name: slackTarget.Name},
			Spec:       v1alpha1.SQSTargetSpec{EventRule: slackTarget.Spec.EventRule},
		}
		if err := c.kubeClient.Create(ctx, sqsTarget); err != nil {
			return nil, fmt.Errorf("creating sqs target, %w", err)
		}
	}
	return sqsTarget, nil
}

type Body struct {
	Detail *v1.Event
}

func Format(ctx context.Context, message *sqs.Message) (io.Reader, error) {
	body := &Body{}

	stripped := strings.ReplaceAll(lo.FromPtr(message.Body), "\\\"", "\"")
	log.FromContext(ctx).Info(stripped)

	if err := json.Unmarshal([]byte(stripped), body); err != nil {
		return nil, fmt.Errorf("unmarshalling json, %w", err)
	}
	b, err := json.Marshal(struct {
		Type           string `json:"type"`
		InvolvedObject string `json:"involvedObject"`
		Reason         string `json:"reason"`
		Message        string `json:"message"`
	}{
		Type:           formatType(body.Detail),
		InvolvedObject: formatInvolvedObject(body.Detail),
		Reason:         body.Detail.Reason,
		Message:        body.Detail.Message,
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
		Owns(&v1alpha1.SQSTarget{}).
		Complete(NewController(m.GetClient()))
}
