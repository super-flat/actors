package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	cache "github.com/patrickmn/go-cache"
	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
)

type NodeDispatcher struct {
	msgQueue    chan *actorsv1.Command
	isReceiving bool
	replies     *cache.Cache
	actors      *cache.Cache
}

func NewNodeDispatcher() *NodeDispatcher {
	// number of messages this node can dispatch at the same time
	bufferSize := 100
	// create actor cache that shuts down actors on eviction
	actorCache := cache.New(time.Second*5, time.Second*15)
	actorCache.OnEvicted(evictActor)
	// reply cache
	replyTimeout := time.Minute * 1
	replyCache := cache.New(replyTimeout, replyTimeout*2)
	// create the dispatcher
	return &NodeDispatcher{
		msgQueue:    make(chan *actorsv1.Command, bufferSize),
		isReceiving: false,
		actors:      actorCache,
		replies:     replyCache,
	}
}

func (x *NodeDispatcher) Send(ctx context.Context, msg *actorsv1.Command) (*actorsv1.Response, error) {
	if !x.isReceiving {
		return nil, errors.New("not ready")
	}
	// set a unique message id
	msg.MessageId = uuid.NewString()
	// create reply channel
	replyChan := make(chan *actorsv1.Response, 1)
	x.replies.SetDefault(msg.GetMessageId(), replyChan)
	// put message into queue
	x.msgQueue <- msg
	// try to read response form reply channel
	resp := <-replyChan
	// clean up reply channel
	x.replies.Delete(msg.GetMessageId())
	return resp, nil
}

func (x *NodeDispatcher) Start() {
	if x.isReceiving {
		return
	}
	go x.process()
	x.isReceiving = true
}

func (x *NodeDispatcher) process() {
	for {
		msg := <-x.msgQueue
		// get or create the actor from cache
		var actor *Actor
		value, exists := x.actors.Get(msg.GetActorId())
		if !exists {
			actor = NewActor(msg.GetActorId())
			fmt.Printf("(dispatcher) creating actor, id=%s\n", actor.ID)
		} else {
			actor, _ = value.(*Actor)
		}
		var replyChan chan *actorsv1.Response
		if val, ok := x.replies.Get(msg.GetMessageId()); ok {
			replyChan, _ = val.(chan *actorsv1.Response)
		}
		actor.AddToMailbox(msg, replyChan)
		// re-write the actor into cache so it resets the expiry
		x.actors.SetDefault(actor.ID, actor)
	}
}

func (x *NodeDispatcher) AwaitTermination() {
	for {
		if x.isReceiving {
			// fmt.Println("running...")
			time.Sleep(time.Second * 3)
		}
	}
}

func evictActor(actorID string, actor interface{}) {
	typedActor, ok := actor.(*Actor)
	if ok {
		typedActor.Stop()
		fmt.Printf("(dispatcher) passivating actor, id='%s'\n", typedActor.ID)
	}
}
