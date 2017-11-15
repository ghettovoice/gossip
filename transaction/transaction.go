package transaction

import (
	"errors"
	"time"

	"fmt"

	"strings"

	"github.com/discoviking/fsm"
	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/timing"
	"github.com/ghettovoice/gossip/transport"
)

const RFC3261MagicCookie = "z9hG4bK"

const (
	T1 = 500 * time.Millisecond
	T2 = 4 * time.Second
)

type Transaction interface {
	log.WithLocalLogger
	Receive(m base.SipMessage)
	Origin() *base.Request
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

func (tx *ServerTransaction) Delete() {
	tx.Log().Debugf("deleting transaction %p from manager %p", tx, tx.tm)
	tx.tm.delTx(tx)
}

func (tx *ClientTransaction) Delete() {
	tx.Log().Debugf("deleting transaction %p from manager %p", tx, tx.tm)
	tx.tm.delTx(tx)
}

type ClientTransaction struct {
	transaction

	tu           chan *base.Response // Channel to transaction user.
	tu_err       chan error          // Channel to report up errors to TU.
	timer_a_time time.Duration       // Current duration of timer A.
	timer_a      timing.Timer
	timer_b      timing.Timer
	timer_d_time time.Duration // Current duration of timer A.
	timer_d      timing.Timer
}

type ServerTransaction struct {
	transaction

	tu      chan *base.Response // Channel to transaction user.
	tu_err  chan error          // Channel to report up errors to TU.
	ack     chan *base.Request  // Channel we send the ACK up on.
	timer_g timing.Timer
	timer_h timing.Timer
	timer_i timing.Timer
}

func (tx *ServerTransaction) Receive(m base.SipMessage) {
	r, ok := m.(*base.Request)
	if !ok {
		tx.Log().Warn("client transaction received request")
	}

	var input fsm.Input = fsm.NO_INPUT
	switch {
	case r.Method == tx.origin.Method:
		input = server_input_request
	case r.Method == base.ACK:
		input = server_input_ack
		tx.ack <- r
	default:
		tx.Log().Warn("invalid message correlated to server transaction.")
	}

	tx.fsm.Spin(input)
}

func (tx *ServerTransaction) Respond(r *base.Response) {
	tx.lastResp = r

	var input fsm.Input
	switch {
	case r.StatusCode < 200:
		input = server_input_user_1xx
	case r.StatusCode < 300:
		input = server_input_user_2xx
	default:
		input = server_input_user_300_plus
	}

	tx.fsm.Spin(input)
}

// Ack returns channel for ACK requests on non-2xx responses - RFC 3261 - 17.1.1.3
func (tx *ServerTransaction) Ack() <-chan *base.Request {
	return (<-chan *base.Request)(tx.ack)
}

func (tx *ClientTransaction) Receive(m base.SipMessage) {
	r, ok := m.(*base.Response)
	if !ok {
		tx.Log().Warn("client transaction received request")
	}

	tx.lastResp = r

	var input fsm.Input
	switch {
	case r.StatusCode < 200:
		input = client_input_1xx
	case r.StatusCode < 300:
		input = client_input_2xx
	default:
		input = client_input_300_plus
	}

	tx.fsm.Spin(input)
}

// Resend the originating request.
func (tx *ClientTransaction) resend() {
	tx.Log().Infof("client transaction %p resending request: %v", tx, tx.origin.Short())
	err := tx.transport.Send(tx.dest, tx.origin)
	if err != nil {
		tx.fsm.Spin(client_input_transport_err)
	}
}

// Pass up the most recently received response to the TU.
func (tx *ClientTransaction) passUp() {
	tx.Log().Infof("client transaction %p passing up response: %v", tx, tx.lastResp.Short())
	tx.tu <- tx.lastResp
}

// Send an error to the TU.
func (tx *ClientTransaction) transportError() {
	tx.Log().Infof("client transaction %p had a transport-level error", tx)
	tx.tu_err <- errors.New("failed to send message")
}

// Inform the TU that the transaction timed out.
func (tx *ClientTransaction) timeoutError() {
	tx.Log().Infof("client transaction %p timed out", tx)
	tx.tu_err <- errors.New("transaction timed out")
}

// Send an automatic ACK - RFC 3261 - 17.1.1.3.
func (tx *ClientTransaction) Ack() {
	ack := base.NewRequest(
		base.ACK,
		tx.origin.Recipient,
		tx.origin.SipVersion(),
		[]base.SipHeader{},
		"",
		tx.Log(),
	)

	// Copy headers from original request.
	// TODO: Safety
	base.CopyHeaders("From", tx.origin, ack)
	base.CopyHeaders("Call-Id", tx.origin, ack)
	base.CopyHeaders("Route", tx.origin, ack)
	cseq := tx.origin.Headers("CSeq")[0].Copy()
	cseq.(*base.CSeq).MethodName = base.ACK
	ack.AddHeader(cseq)
	via := tx.origin.Headers("Via")[0].Copy()
	ack.AddHeader(via)

	// Copy headers from response.
	base.CopyHeaders("To", tx.lastResp, ack)

	// Send the ACK.
	tx.transport.Send(tx.dest, ack)
}

// Return the channel we send responses on.
func (tx *ClientTransaction) Responses() <-chan *base.Response {
	return (<-chan *base.Response)(tx.tu)
}

// Return the channel we send errors on.
func (tx *ClientTransaction) Errors() <-chan error {
	return (<-chan error)(tx.tu_err)
}

type txId []interface{}

// MakeId returns transaction key - RFC 17.2.3.
func MakeId(msg base.SipMessage) (txId, error) {
	firstViaHop, err := msg.ViaHop()
	if err != nil {
		return nil, fmt.Errorf("couldn't create transaction id: %s", err)
	}

	switch msg := msg.(type) {
	case *base.Request:
		method := msg.Method
		if method == base.ACK {
			method = base.INVITE
		}

		if branch, err := msg.Branch(); err == nil {
			if branchStr, ok := branch.(base.String); ok && branchStr.String() != "" &&
				strings.HasPrefix(branchStr.String(), RFC3261MagicCookie) &&
				strings.TrimPrefix(branchStr.String(), RFC3261MagicCookie) != "" {
				// RFC3261 compliant
				return txId{
					branch,
					firstViaHop,
					method,
				}, nil
			}
		}
		// RFC 2543 compliant
		fromTag, err := msg.FromTag()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}
		toTag, err := msg.ToTag()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}
		callId, err := msg.CallId()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}
		cseq, err := msg.CSeq()
		if err != nil {
			return nil, fmt.Errorf("couldn't create transaction id: %s", err)
		}

		return txId{
			msg.Recipient.String(),
			toTag,
			fromTag,
			callId,
			firstViaHop,
			method,
			cseq.SeqNo,
		}, nil
	case *base.Response:
		// todo
	default:
		return nil, fmt.Errorf("unsupported message type")
	}
}
