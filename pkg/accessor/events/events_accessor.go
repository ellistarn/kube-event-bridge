package events

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/eventbridge/eventbridgeiface"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

type eventsAccessor struct {
	client eventbridgeiface.EventBridgeAPI
}

// https://docs.aws.amazon.com/sdk-for-go/api/service/eventbridge/#EventBridge.PutRule
func (this *eventsAccessor) CreateEventRule(name, eventBusName, eventPattern *string) (*string, error) {
	putEventRuleInput := &eventbridge.PutRuleInput{
		Name:         name,
		EventBusName: eventBusName,
		EventPattern: eventPattern,
	}

	response, err := this.client.PutRule(putEventRuleInput)

	if err != nil {
		return nil, err
	}

	ruleArn := response.RuleArn
	return ruleArn, nil
}

func (this *eventsAccessor) PutEvents(eventBusName, detail, detailType, source, resourceName *string) (*eventbridge.PutEventsOutput, error) {

	resources := make([]*string, 1)
	resources[0] = resourceName
	putEventRequestEntryInput := &eventbridge.PutEventsRequestEntry{
		Detail:       detail,
		DetailType:   detailType,
		EventBusName: eventBusName,
		Resources:    resources,
		Source:       source,
	}

	//var putEventRequestEntryInputArr []*eventbridge.PutEventsRequestEntry
	putEventRequestEntryInputArr := make([]*eventbridge.PutEventsRequestEntry, 1)
	putEventRequestEntryInputArr[0] = putEventRequestEntryInput

	putEventsInputRequest := &eventbridge.PutEventsInput{
		Entries: putEventRequestEntryInputArr,
	}

	result, err := this.client.PutEvents(putEventsInputRequest)

	if err != nil {
		return nil, err
	}

	return result, err
}

func (this *eventsAccessor) PutTargets(eventrule, eventbus, arn *string) error {

	random := uuid.New().String()
	target := &eventbridge.Target{
		Id:  &random,
		Arn: arn,
	}

	targets := make([]*eventbridge.Target, 1)
	targets[0] = target

	putTargetsInput := &eventbridge.PutTargetsInput{
		EventBusName: eventbus,
		Rule:         eventrule,
		Targets:      targets,
	}
	_, err := this.client.PutTargets(putTargetsInput)

	if err != nil {
		return err
	}

	return nil

}

func NewEventsAccessor() *eventsAccessor {
	client := eventbridge.New(lo.Must(session.NewSession()))
	return &eventsAccessor{
		client: client,
	}
}
