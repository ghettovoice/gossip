package transaction

import (
	"fmt"
	"time"

	"github.com/discoviking/fsm"
	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/transport"
)

const (
	T1      = 500 * time.Millisecond
	T2      = 4 * time.Second
	T4      = 5 * time.Second
	Timer_A = T1
	Timer_B = 64 * T1
	Timer_D = 32 * time.Second
	Timer_H = 64 * T1
)

type Transaction interface {
	log.WithLocalLogger
	Receive(m base.SipMessage)
	Origin() *base.Request
	LastResponse() *base.Response
	Destination() string
	Transport() transport.Manager
	Delete()
}

type transaction struct {
	fsm       *fsm.FSM       // FSM which governs the behavior of this transaction.
	origin    *base.Request  // Request that started this transaction.
	lastResp  *base.Response // Most recently received message.
	dest      string         // Of the form hostname:port
	transport transport.Manager
	tm        *Manager
	lastErr   error
}

func (tx *transaction) Log() log.Logger {
	return tx.origin.Log().WithField("tx-ptr", fmt.Sprintf("%p", tx))
}

func (tx *transaction) Origin() *base.Request {
	return tx.origin
}

func (tx *transaction) Destination() string {
	return tx.dest
}

func (tx *transaction) Transport() transport.Manager {
	return tx.transport
}

func (tx *transaction) LastResponse() *base.Response {
	return tx.lastResp
}
