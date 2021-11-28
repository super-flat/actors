package cmd

import (
	"context"
	"fmt"
	"log"
	"sync"
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

	metrics := &counter{
		calls:    0,
		duration: time.Millisecond * 0,
		mtx:      &sync.Mutex{},
	}

	go doReporting(metrics)

	go sendMessages(nd, "sender-0", "actor-0", time.Millisecond*10, metrics, 20*time.Minute)

	for i := 0; i < 20; i++ {
		senderID := "sender-1"
		actorID := fmt.Sprintf("actor-%d", i)
		go sendMessages(nd, senderID, actorID, time.Millisecond*10, metrics, 20*time.Second)
		time.Sleep(3 * time.Second)
	}

	nd.AwaitTermination()
}

func sendMessages(nd *actors.Dispatcher, senderID string, actorID string, sleepTime time.Duration, metrics *counter, lifespan time.Duration) {
	loopCount := 0

	outerStart := time.Now()

	for time.Since(outerStart) < lifespan {
		msg := wrapperspb.String(fmt.Sprintf("message %d from %s", loopCount, senderID))
		start := time.Now()
		_, _ = nd.Send(context.Background(), actorID, msg)
		metrics.Add(time.Since(start))
		loopCount += 1
		time.Sleep(sleepTime)
	}
}

func doReporting(metrics *counter) {
	for {
		time.Sleep(500 * time.Millisecond)
		metrics.Report()
	}
}

type counter struct {
	calls    int64
	duration time.Duration
	mtx      *sync.Mutex
}

func (c *counter) Add(t time.Duration) {
	c.mtx.Lock()
	c.calls += 1
	c.duration = c.duration + t
	c.mtx.Unlock()
}

func (c *counter) Report() {
	c.mtx.Lock()
	avg := c.duration.Milliseconds() / c.calls
	log.Printf("[Metrics] avg=%dms, calls=%d\n", avg, c.calls)
	c.mtx.Unlock()
}
