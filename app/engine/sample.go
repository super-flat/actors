package engine

import (
	"context"
	"fmt"
	"time"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func Sample() {
	nd := NewNodeDispatcher()
	nd.Start()
	go sendMessages(nd, "sender-1")
	go sendMessages(nd, "sender-2")
	nd.AwaitTermination()
}

func sendMessages(nd *NodeDispatcher, sender string) {
	counter := 0

	for {
		msg := wrapperspb.String(fmt.Sprintf("message %d from %s", counter, sender))
		msgAny, _ := anypb.New(msg)
		cmd := &actorsv1.Command{
			ActorId: "actor-1",
			Message: msgAny,
		}
		fmt.Printf("(%s) sending msg to actor_id=%s, msg=%s\n", sender, cmd.GetActorId(), msg.GetValue())
		response, err := nd.Send(context.Background(), cmd)
		if err != nil {
			fmt.Printf("send failed, counter=%d, err=%s\n", counter, err.Error())
		}

		if response != nil {
			respMsg := &wrapperspb.StringValue{}
			_ = response.GetMessage().UnmarshalTo(respMsg)
			fmt.Printf("(%s) received actor_id=%s, msg=%s\n", sender, response.GetActorId(), respMsg.GetValue())
		}

		counter += 1
		time.Sleep(time.Second * 10)
	}
}
