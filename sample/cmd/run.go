package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/super-flat/actors/engine"
	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func init() {
	rootCmd.AddCommand(runCMD)
}

var runCMD = &cobra.Command{
	Use: "run",
	Run: func(cmd *cobra.Command, args []string) {
		Sample()
	},
}

func Sample() {
	nd := engine.NewActorDispatcher()
	nd.Start()
	go sendMessages(nd, "sender-1", "actor-1", time.Second*20)
	go sendMessages(nd, "sender-2", "actor-1", time.Second*15)
	go sendMessages(nd, "sender-3", "actor-2", time.Second*3)
	nd.AwaitTermination()
}

func sendMessages(nd *engine.ActorDispatcher, senderID string, actorID string, sleepTime time.Duration) {
	counter := 0

	for {
		msg := wrapperspb.String(fmt.Sprintf("message %d from %s", counter, senderID))
		msgAny, _ := anypb.New(msg)
		cmd := &actorsv1.Command{
			ActorId: actorID,
			Message: msgAny,
		}
		fmt.Printf("(%s) sending msg to actor_id=%s, msg=%s\n", senderID, cmd.GetActorId(), msg.GetValue())
		response, err := nd.Send(context.Background(), cmd)
		if err != nil {
			fmt.Printf("send failed, counter=%d, err=%s\n", counter, err.Error())
		}

		if response != nil {
			respMsg := &wrapperspb.StringValue{}
			_ = response.GetMessage().UnmarshalTo(respMsg)
			fmt.Printf("(%s) received actor_id=%s, msg=%s\n", senderID, response.GetActorId(), respMsg.GetValue())
		}

		counter += 1
		time.Sleep(sleepTime)
	}
}
