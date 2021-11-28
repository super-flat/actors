package actors

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

// CommandWrapper wraps the actual command sent to the actor and
// the context
type CommandWrapper struct {
	CommandCtx context.Context
	Command    proto.Message
	ReplyChan  chan<- proto.Message
}

// Mailbox has a mailbox and can process messages one at a time
type Mailbox struct {
	ID                string
	mailbox           chan *CommandWrapper
	msgCount          int
	stop              chan bool
	lastUpdated       time.Time
	acceptingMessages bool
	mtx               sync.RWMutex
	actor             Actor
}

// NewMailbox returns a new actor
func NewMailbox(ctx context.Context, ID string, actorFactory ActorFactory) *Mailbox {
	// TODO set the mailbox size
	// mailboxSize := 10
	actor := actorFactory(ID)
	// create the actor mailbox
	mailbox := &Mailbox{
		ID:                ID,
		mailbox:           make(chan *CommandWrapper),
		stop:              make(chan bool),
		lastUpdated:       time.Now(),
		acceptingMessages: true,
		actor:             actor,
	}
	// initialize the actor and start the actor loop
	mailbox.selfInit(ctx)
	// start the actor loop
	go mailbox.receiverLoop()
	// return the mailbox
	return mailbox
}

// IdleTime returns how long the actor has been idle as a time.Duration
func (x *Mailbox) IdleTime() time.Duration {
	return time.Since(x.lastUpdated)
}

// Send sends a message to the actors' mailbox to be processed and
// supplies an optional reply channel for responses to the sender
func (x *Mailbox) Send(ctx context.Context, msg proto.Message) (success bool, replyChan <-chan proto.Message) {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Actor.Mailbox.Send")
	defer span.End()
	// acquire a lock
	x.mtx.Lock()
	defer x.mtx.Unlock()
	// process
	if !x.acceptingMessages {
		return false, nil
	}
	// set update time for activity
	x.lastUpdated = time.Now()
	// create the reply chan
	replyTo := make(chan proto.Message, 1)
	// if successfully push to channel, return true, else false
	x.mailbox <- &CommandWrapper{
		CommandCtx: spanCtx,
		Command:    msg,
		ReplyChan:  replyTo,
	}
	// return the response
	return true, replyTo
}

// Stop the actor
func (x *Mailbox) Stop() {
	// acquire a lock
	x.mtx.Lock()
	// stop future messages
	x.acceptingMessages = false
	// unlock
	x.mtx.Unlock()
	// wait for no more messages
	for len(x.mailbox) != 0 {
		// log.Printf("[mailbox] waiting for mailbox empty, len=%d\n", len(x.mailbox))
		time.Sleep(time.Millisecond)
	}
	// begin shutdown
	log.Printf("[mailbox] (%s) shutting down\n", x.ID)
	x.stop <- true
}

// the actor loop
func (x *Mailbox) receiverLoop() {
	for {
		select {
		case <-x.stop:
			return
		case received := <-x.mailbox:
			// let the actor process the command
			err := x.actor.Receive(received.CommandCtx, received.Command, received.ReplyChan)
			if err != nil {
				log.Printf("[mailbox] error handling message, messageType=%s, err=%s\n", received.Command.ProtoReflect().Descriptor().FullName(), err.Error())
			}
			// increase the message counter
			// TODO add this prometheus
			x.msgCount += 1
		}
	}
}

// process runs in the background and processes all messages in the mailbox
func (x *Mailbox) selfInit(ctx context.Context) {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Actor.Mailbox.Init")
	defer span.End()
	// initialize the actor
	if err := x.actor.Init(spanCtx); err != nil {
		log.Panicf("[mailbox] failed to initialize actor, err=%s", err.Error())
	}
}
