package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
)

// ActorMailbox has a mailbox and can process messages one at a time
type ActorMailbox struct {
	ID                string
	mailbox           chan *ActorMessage
	msgCount          int
	stop              chan bool
	lastUpdated       time.Time
	acceptingMessages bool
	mtx               sync.RWMutex
	actor             Actor
}

// NewActorMailbox returns a new actor
func NewActorMailbox(ID string, actorFactory ActorFactory) *ActorMailbox {
	mailboxSize := 10
	actor := actorFactory.CreateActor(ID)
	mailbox := &ActorMailbox{
		ID:                ID,
		mailbox:           make(chan *ActorMessage, mailboxSize),
		stop:              make(chan bool, 1),
		lastUpdated:       time.Now(),
		acceptingMessages: true,
		actor:             actor,
	}
	go mailbox.process()
	fmt.Printf("(dispatcher) creating actor, id=%s\n", mailbox.ID)
	return mailbox
}

// IdleTime returns how long the actor has been idle as a time.Duration
func (x *ActorMailbox) IdleTime() time.Duration {
	return time.Since(x.lastUpdated)
}

// AddToMailbox adds a message to the actors mailbox to be processed and
// supplies an optional reply channel for responses to the sender
func (x *ActorMailbox) AddToMailbox(msg *actorsv1.Command) (success bool, replyChan <-chan *actorsv1.Response) {
	// acquire a lock
	x.mtx.Lock()
	defer x.mtx.Unlock()
	// process
	if !x.acceptingMessages {
		return false, nil
	}
	// set update time for activity
	x.lastUpdated = time.Now()
	// create message
	replyTo := make(chan *actorsv1.Response, 1)
	wrapped := &ActorMessage{
		Payload: msg,
		ReplyTo: replyTo,
	}
	// if successfully push to channel, return true, else false
	x.mailbox <- wrapped
	return true, replyTo
}

// Stop the actor
func (x *ActorMailbox) Stop() {
	// acquire a lock
	x.mtx.Lock()
	// stop future messages
	x.acceptingMessages = false
	// unlock
	x.mtx.Unlock()
	// wait for no more messages
	for len(x.mailbox) != 0 {
		fmt.Printf("waiting for mailbox empty, len=%d\n", len(x.mailbox))
		time.Sleep(time.Millisecond)
	}
	// begin shutdown
	fmt.Printf("(%s) shutting down\n", x.ID)
	x.stop <- true
}

// process runs in the background and processes all messages in the mailbox
func (x *ActorMailbox) process() {
	for {
		select {
		case <-x.stop:
			return
		case wrapper := <-x.mailbox:
			// run handler
			err := x.actor.Receive(context.Background(), wrapper)
			if err != nil {
				fmt.Printf("error handling message, id=%s, err=%s\n", wrapper.Payload.MessageId, err.Error())
			}
			x.msgCount += 1
		default:
			continue
		}
	}
}
