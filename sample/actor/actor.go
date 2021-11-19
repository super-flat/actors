package actor

import (
	"context"
	"fmt"

	"github.com/super-flat/actors/engine"
	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type SampleActor struct {
	ID string
}

func (x SampleActor) Receive(ctx context.Context, msg *engine.ActorMessage) error {
	msgString := &wrapperspb.StringValue{}
	if err := msg.Payload.GetMessage().UnmarshalTo(msgString); err != nil {
		fmt.Printf("failed to unmarshal, %s\n", err.Error())
		return err
	} else {
		fmt.Printf("(%s) received msg='%s'\n", msg.Payload.GetActorId(), msgString.GetValue())
		respMsg := wrapperspb.String(fmt.Sprintf("reply %s", msgString.GetValue()))
		respAny, _ := anypb.New(respMsg)
		resp := &actorsv1.Response{
			ActorId: msg.Payload.ActorId,
			Message: respAny,
		}
		msg.ReplyTo <- resp
		return nil
	}
}

type SampleActorFactory struct {
}

func (x SampleActorFactory) CreateActor(actorID string) engine.Actor {
	return &SampleActor{ID: actorID}
}

func NewSampleActorFactory() *SampleActorFactory {
	return &SampleActorFactory{}
}
