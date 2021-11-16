package engine

import (
	"fmt"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Actor struct {
	ID       string
	mailbox  chan *mailboxMessage
	state    *anypb.Any
	msgCount int
	stop     chan bool
}

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

func (x *Actor) AddToMailbox(msg *actorsv1.Command, reply chan<- *actorsv1.Response) error {
	wrapped := &mailboxMessage{
		msg:     msg,
		replyTo: reply,
	}
	x.mailbox <- wrapped
	return nil
}

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

func (x *Actor) Stop() {
	fmt.Printf("(%s) shutting down", x.ID)
	x.stop <- true
}

type mailboxMessage struct {
	msg     *actorsv1.Command
	replyTo chan<- *actorsv1.Response
}
