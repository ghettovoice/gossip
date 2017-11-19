package transport

import "github.com/ghettovoice/gossip/message"

const (
	bufSize            uint16 = 65535
	listenersQueueSize uint16 = 10000
)

// Layer is an transport layer RFC 3261 - 18.
type Layer interface {
	Listen(addr string) error
	Send(addr string, msg message.SipMessage) error
	Stop()
	// Messages returns channel with incoming messages
	Messages() <-chan message.SipMessage
	// Errors returns transport layers errors
	Errors() <-chan error
}

type layer struct {
	protocols map[Protocol]Protocol
	messages  chan message.SipMessage
	errors    chan error
}

func (tl *layer) Messages() <-chan message.SipMessage {
	return tl.messages
}

func (tl *layer) Errors() <-chan error {
	return tl.errors
}
