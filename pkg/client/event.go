package client

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventcontracts "github.com/hatchet-dev/hatchet/internal/services/ingestor/contracts"
	"github.com/hatchet-dev/hatchet/pkg/validator"
)

type PushOpFunc func(*eventcontracts.PushEventRequest) error

type BulkPushOpFunc func(*eventcontracts.BulkPushEventRequest) error

type EventClient interface {
	Push(ctx context.Context, eventKey string, payload interface{}, options ...PushOpFunc) error

	BulkPush(ctx context.Context, payloads []EventWithAdditionalMetadata, options ...BulkPushOpFunc) error

	PutLog(ctx context.Context, stepRunId, msg string) error

	PutStreamEvent(ctx context.Context, stepRunId string, message []byte) error
}

type EventWithAdditionalMetadata struct {
	Event              interface{}       `json:"event"`
	AdditionalMetadata map[string]string `json:"metadata"`
	Key                string            `json:"key"`
}

type eventClientImpl struct {
	client eventcontracts.EventsServiceClient

	tenantId string

	namespace string

	l *zerolog.Logger

	v validator.Validator

	ctx *contextLoader
}

func newEvent(conn *grpc.ClientConn, opts *sharedClientOpts) EventClient {
	return &eventClientImpl{
		client:    eventcontracts.NewEventsServiceClient(conn),
		tenantId:  opts.tenantId,
		namespace: opts.namespace,
		l:         opts.l,
		v:         opts.v,
		ctx:       opts.ctxLoader,
	}
}

func WithEventMetadata(metadata interface{}) PushOpFunc {
	return func(r *eventcontracts.PushEventRequest) error {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return err
		}

		metadataString := string(metadataBytes)

		r.AdditionalMetadata = &metadataString

		return nil
	}
}

func (a *eventClientImpl) Push(ctx context.Context, eventKey string, payload interface{}, options ...PushOpFunc) error {

	request := eventcontracts.PushEventRequest{
		Key:            a.namespace + eventKey,
		EventTimestamp: timestamppb.Now(),
	}

	payloadBytes, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	request.Payload = string(payloadBytes)

	for _, optionFunc := range options {
		err = optionFunc(&request)
		if err != nil {
			return err
		}
	}

	_, err = a.client.Push(a.ctx.newContext(ctx), &request)

	if err != nil {
		return err
	}

	return nil
}

func (a *eventClientImpl) BulkPush(ctx context.Context, payload []EventWithAdditionalMetadata, options ...BulkPushOpFunc) error {

	request := eventcontracts.BulkPushEventRequest{}

	var events []*eventcontracts.PushEventRequest

	for _, p := range payload {

		ePayload, err := json.Marshal(p.Event)
		if err != nil {
			return err
		}
		eMetadata, err := json.Marshal(p.AdditionalMetadata)
		if err != nil {
			return err
		}
		eMetadataString := string(eMetadata)

		events = append(events, &eventcontracts.PushEventRequest{
			Key:                a.namespace + p.Key,
			EventTimestamp:     timestamppb.Now(),
			Payload:            string(ePayload),
			AdditionalMetadata: &eMetadataString,
		})
	}

	request.Events = events

	_, err := a.client.BulkPush(a.ctx.newContext(ctx), &request)

	if err != nil {
		return err
	}

	return nil
}

func (a *eventClientImpl) PutLog(ctx context.Context, stepRunId, msg string) error {
	_, err := a.client.PutLog(a.ctx.newContext(ctx), &eventcontracts.PutLogRequest{
		CreatedAt: timestamppb.Now(),
		StepRunId: stepRunId,
		Message:   msg,
	})

	return err
}

func (a *eventClientImpl) PutStreamEvent(ctx context.Context, stepRunId string, message []byte) error {
	_, err := a.client.PutStreamEvent(a.ctx.newContext(ctx), &eventcontracts.PutStreamEventRequest{
		CreatedAt: timestamppb.Now(),
		StepRunId: stepRunId,
		Message:   message,
	})

	return err
}
