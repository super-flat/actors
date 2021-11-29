package actors

import (
	"context"
	"log"
	"math"
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
	// set the mailbox size
	// TODO: make configurable
	mailboxSize := 10
	// create the inner actor
	actor := actorFactory(ID)
	// create the actor mailbox
	mailbox := &Mailbox{
		ID:                ID,
		mailbox:           make(chan *CommandWrapper, mailboxSize),
		stop:              make(chan bool),
		lastUpdated:       time.Now(),
		acceptingMessages: true,
		actor:             actor,
	}
	// async initialize the actor and start processing messages
	go mailbox.run(ctx)
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

// run the actor loop
func (x *Mailbox) run(bootCtx context.Context) {
	// run the actor initialization
	x.init(bootCtx)
	// run the process loop
	x.process()
}

// process incoming messages until stop signal
func (x *Mailbox) process() {
	// run the processing loop
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

// init runs the actor initialization
func (x *Mailbox) init(ctx context.Context) {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Actor.Mailbox.Init")
	defer span.End()
	// initialize the actor
	loopCount := 0
	for {
		if err := x.actor.Init(spanCtx); err != nil {
			log.Printf("[mailbox] failed to initialize actor, attempt=%d, err=%s", loopCount+1, err.Error())
			// exponential backoff 10% more each time
			x.sleepBackoff(5*time.Millisecond, 1*time.Second, 1.1, loopCount)
			loopCount += 1
		} else {
			break
		}
	}
}

// sleepBackoff helps sleep with exponential backoff
func (x *Mailbox) sleepBackoff(base time.Duration, maxBackoff time.Duration, factor float64, iteration int) {
	// compute factor ^ iteration
	backoffFactor := math.Pow(factor, float64(iteration))
	// compute milliseconds to backoff
	backoffMs := int64(float64(base.Milliseconds()) * backoffFactor)
	backoffDuration := time.Duration(backoffMs) * time.Millisecond
	log.Printf("backoff %v", backoffDuration)
	// only backoff up to max backoff
	if backoffDuration > maxBackoff {
		backoffDuration = maxBackoff
	}
	// sleep
	time.Sleep(backoffDuration)
}
