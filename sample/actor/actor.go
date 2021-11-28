package actor

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// SampleActor implements engine.Actor
type SampleActor struct {
	ID string
}

// NewSampleActor creates a new SampleActor
func NewSampleActor(actorID string) *SampleActor {
	log.Printf("[SampleActor] (%s) booting\n", actorID)
	return &SampleActor{ID: actorID}
}

// Receive handles new messages
func (x SampleActor) Receive(ctx context.Context, command proto.Message, replyToChan chan<- proto.Message) error {
	msgString := command.(*wrapperspb.StringValue)
	// fmt.Printf("(%s) received msg='%s'\n", x.ID, msgString.GetValue())
	replyToChan <- wrapperspb.String(fmt.Sprintf("reply %s", msgString.GetValue()))
	return nil
}

func (x *SampleActor) Init(ctx context.Context) error {
	return nil
}
