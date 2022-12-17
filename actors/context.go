package actors

import (
	"context"
	"sync"

	"google.golang.org/protobuf/proto"
)

// ReceiveContext is the context that is used by the actor to receive messages
type ReceiveContext interface {
	// Context returns the context attached to the message
	Context() context.Context
	// Sender of the message
	Sender() PID
	// Self represents the actor receiving the message.
	Self() PID
	// Message is the actual message sent
	Message() proto.Message
	// Response sets the message response
	// Use this method within the Actor.Receive method of the actor to sets a reply
	// This can only be used when we are request-response pattern. When it is an async commnunication
	// this operation will amount to nothing.
	Response(resp proto.Message)
}

type receiveContext struct {
	ctx            context.Context
	message        proto.Message
	sender         PID
	response       chan proto.Message
	recipient      PID
	mu             sync.Mutex
	isAsyncMessage bool
}

// Self returns the receiver PID of the message
func (m *receiveContext) Self() PID {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recipient
}

// Response sets the message response
func (m *receiveContext) Response(resp proto.Message) {
	m.mu.Lock()
	// only set a response when the message is sync message
	if !m.isAsyncMessage {
		m.response <- resp
		m.mu.Unlock()
		close(m.response)
		return
	}
	m.mu.Unlock()
}

// Context represents the context attached to the message
func (m *receiveContext) Context() context.Context {
	m.mu.Lock()
	ctx := m.ctx
	m.mu.Unlock()
	return ctx
}

// Sender of the message
func (m *receiveContext) Sender() PID {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sender
}

// Message is the actual message sent
func (m *receiveContext) Message() proto.Message {
	m.mu.Lock()
	content := m.message
	m.mu.Unlock()
	return content
}