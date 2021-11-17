package engine

import (
	"errors"
	"fmt"
	"time"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Actor has a mailbox and can process messages one at a time
type Actor struct {
	ID                string
	mailbox           chan *mailboxMessage
	msgCount          int
	stop              chan bool
	lastUpdated       time.Time
	acceptingMessages bool
	// state    *anypb.Any
}

// NewActor returns a new actor
func NewActor(ID string) *Actor {
	mailboxSize := 100
	a := &Actor{
		ID:                ID,
		mailbox:           make(chan *mailboxMessage, mailboxSize),
		stop:              make(chan bool, 1),
		lastUpdated:       time.Now(),
		acceptingMessages: true,
	}
	go a.process()
	fmt.Printf("(dispatcher) creating actor, id=%s\n", a.ID)
	return a
}

// IdleTime returns how long the actor has been idle as a time.Duration
func (x Actor) IdleTime() time.Duration {
	return time.Since(x.lastUpdated)
}

// AddToMailbox adds a message to the actors mailbox to be processed and
// supplies an optional reply channel for responses to the sender
func (x *Actor) AddToMailbox(msg *actorsv1.Command, reply chan<- *actorsv1.Response) error {
	if !x.acceptingMessages {
		return errors.New("actor shutting down")
	}
	wrapped := &mailboxMessage{
		msg:     msg,
		replyTo: reply,
	}
	x.mailbox <- wrapped
	return nil
}

// Stop the actor
func (x *Actor) Stop() {
	fmt.Printf("(%s) shutting down\n", x.ID)
	x.stop <- true
}

// process runs in the background and processes all messages in the mailbox
func (x *Actor) process() {
	for {
		select {
		case <-x.stop:
			x.acceptingMessages = false
			return
		case wrapper := <-x.mailbox:
			x.lastUpdated = time.Now()
			// run handler
			response, err := handleMessage(wrapper.msg)
			if err != nil {
				fmt.Printf("error handling message, id=%s, err=%s\n", wrapper.msg.MessageId, err.Error())
			} else {
				if response != nil && wrapper.replyTo != nil {
					wrapper.replyTo <- response
				}
			}

			x.msgCount += 1
		}
	}
}

// handleMessage sample handling messages
func handleMessage(msg *actorsv1.Command) (*actorsv1.Response, error) {
	msgString := &wrapperspb.StringValue{}
	if err := msg.GetMessage().UnmarshalTo(msgString); err != nil {
		fmt.Printf("failed to unmarshal, %s\n", err.Error())
		return nil, err
	} else {
		fmt.Printf("(%s) received msg='%s'\n", msg.GetActorId(), msgString.GetValue())
		respMsg := wrapperspb.String(fmt.Sprintf("reply %s", msgString.GetValue()))
		respAny, _ := anypb.New(respMsg)
		resp := &actorsv1.Response{
			ActorId: msg.ActorId,
			Message: respAny,
		}
		return resp, nil
	}
}

// mailboxMessage wraps messaegs with the reply-to channel
type mailboxMessage struct {
	msg     *actorsv1.Command
	replyTo chan<- *actorsv1.Response
}
