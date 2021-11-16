package engine

import (
	"fmt"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Actor has a mailbox and can process messages one at a time
type Actor struct {
	ID       string
	mailbox  chan *mailboxMessage
	msgCount int
	stop     chan bool
	// state    *anypb.Any
}

// NewActor returns a new actor
func NewActor(ID string) *Actor {
	mailboxSize := 100
	a := &Actor{
		ID:      ID,
		mailbox: make(chan *mailboxMessage, mailboxSize),
		stop:    make(chan bool, 1),
	}
	go a.process()
	return a
}

// AddToMailbox adds a message to the actors mailbox to be processed and
// supplies an optional reply channel for responses to the sender
func (x *Actor) AddToMailbox(msg *actorsv1.Command, reply chan<- *actorsv1.Response) error {
	wrapped := &mailboxMessage{
		msg:     msg,
		replyTo: reply,
	}
	x.mailbox <- wrapped
	return nil
}

// process runs in the background and processes all messages in the mailbox
func (x *Actor) process() {
	for {
		select {
		case <-x.stop:
			return
		case wrapper := <-x.mailbox:
			msg := wrapper.msg
			msgString := &wrapperspb.StringValue{}
			if err := msg.GetMessage().UnmarshalTo(msgString); err != nil {
				fmt.Printf("failed to unmarshal, %s\n", err.Error())
			} else {
				fmt.Printf("(%s) received msg='%s', count=%d\n", msg.GetActorId(), msgString.GetValue(), x.msgCount)
				if wrapper.replyTo != nil {
					respMsg := wrapperspb.String(fmt.Sprintf("reply %s", msgString.GetValue()))
					respAny, _ := anypb.New(respMsg)
					resp := &actorsv1.Response{
						ActorId: x.ID,
						Message: respAny,
					}
					wrapper.replyTo <- resp
				}
			}
			x.msgCount += 1
		}
	}
}

// Stop the actor
func (x *Actor) Stop() {
	fmt.Printf("(%s) shutting down\n", x.ID)
	x.stop <- true
}

// mailboxMessage wraps messaegs with the reply-to channel
type mailboxMessage struct {
	msg     *actorsv1.Command
	replyTo chan<- *actorsv1.Response
}
