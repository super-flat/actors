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
func NewMailbox(ID string, actorFactory ActorFactory) *Mailbox {
	// set the mailbox size
	mailboxSize := 10
	actor := actorFactory(ID)
	mailbox := &Mailbox{
		ID:                ID,
		mailbox:           make(chan *CommandWrapper, mailboxSize),
		stop:              make(chan bool, 10),
		lastUpdated:       time.Now(),
		acceptingMessages: true,
		actor:             actor,
	}
	go mailbox.process()
	log.Printf("[mailbox] creating actor, id=%s\n", mailbox.ID)
	return mailbox
}

// IdleTime returns how long the actor has been idle as a time.Duration
func (x *Mailbox) IdleTime() time.Duration {
	return time.Since(x.lastUpdated)
}

// AddToMailbox adds a message to the actors' mailbox to be processed and
// supplies an optional reply channel for responses to the sender
func (x *Mailbox) AddToMailbox(ctx context.Context, msg proto.Message) (success bool, replyChan <-chan proto.Message) {
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
		CommandCtx: ctx,
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
		log.Printf("[mailbox] waiting for mailbox empty, len=%d\n", len(x.mailbox))
		time.Sleep(time.Millisecond)
	}
	// begin shutdown
	log.Printf("[mailbox] (%s) shutting down\n", x.ID)
	x.stop <- true
}

// process runs in the background and processes all messages in the mailbox
func (x *Mailbox) process() {
	for {
		select {
		case <-x.stop:
			return
		case wrapper := <-x.mailbox:
			// let the actor process the command
			err := x.actor.Receive(wrapper.CommandCtx, wrapper.Command, wrapper.ReplyChan)
			if err != nil {
				log.Printf("[mailbox] error handling message, messageType=%s, err=%s\n", wrapper.Command.ProtoReflect().Descriptor().FullName(), err.Error())
			}
			x.msgCount += 1
		default:
			continue
		}
	}
}
