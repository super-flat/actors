package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/super-flat/actors/actors"
	"github.com/super-flat/actors/sample/actor"
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
	actorFactory := func(actorID string) actors.Actor {
		return actor.NewSampleActor(actorID)
	}

	nd := actors.NewActorDispatcher(actorFactory)
	nd.Start()
	go sendMessages(nd, "sender-1", "actor-1", time.Second*20)
	go sendMessages(nd, "sender-2", "actor-1", time.Second*15)
	go sendMessages(nd, "sender-3", "actor-2", time.Second*3)
	nd.AwaitTermination()
}

func sendMessages(nd *actors.Dispatcher, senderID string, actorID string, sleepTime time.Duration) {
	counter := 0

	for {
		msg := wrapperspb.String(fmt.Sprintf("message %d from %s", counter, senderID))
		fmt.Printf("(%s) sending msg to actor_id=%s, msg=%s\n", senderID, actorID, msg.GetValue())
		response, err := nd.Send(context.Background(), actorID, msg)
		if err != nil {
			fmt.Printf("send failed, counter=%d, err=%s\n", counter, err.Error())
		}

		if response != nil {
			// cast the response to StringValue
			actualResp := response.(*wrapperspb.StringValue)
			fmt.Printf("(%s) received actor_id=%s, msg=%s\n", senderID, actorID, actualResp.GetValue())
		}

		counter += 1
		time.Sleep(sleepTime)
	}
}
