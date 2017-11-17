package transaction

import (
	"github.com/discoviking/fsm"
	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/timing"
)

// ServerTransaction describes SIP server transaction.
type ServerTransaction struct {
	transaction

	tu      chan *base.Response // Channel to transaction user.
	tu_err  chan error          // Channel to report up errors to TU.
	ack     chan *base.Request  // Channel we send the ACK up on.
	timer_g timing.Timer
	timer_h timing.Timer
	timer_i timing.Timer
}

func (tx *ServerTransaction) Delete() {
	tx.Log().Debugf("deleting transaction %p from manager %p", tx, tx.tm)
	err := tx.tm.delServerTx(tx)
	if err != nil {
		tx.Log().Warn(err)
		return
	}
}

func (tx *ServerTransaction) Receive(msg base.SipMessage) {
	req, ok := msg.(*base.Request)
	if !ok {
		tx.Log().Errorf("server transaction %p received wrong message %s, request expected", tx, msg.Short())
		return
	}

	var input fsm.Input = fsm.NO_INPUT
	switch {
	case req.Method == tx.origin.Method:
		input = server_input_request
	case req.Method == base.ACK: // ACK for non-2xx response
		input = server_input_ack
		tx.ack <- req
	default:
		tx.Log().Errorf("invalid message %s correlated to server transaction %p", req.Short(), tx)
		return
	}

	tx.fsm.Spin(input)
}

func (tx *ServerTransaction) Respond(res *base.Response) {
	tx.lastResp = res

	var input fsm.Input
	switch {
	case res.IsProvisional():
		input = server_input_user_1xx
	case res.IsSuccess():
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

// Return the channel we send errors on.
func (tx *ServerTransaction) Errors() <-chan error {
	return (<-chan error)(tx.tu_err)
}

// Trying sends 100 Trying response - RFC 3261 - 17.2.1.
func (tx *ServerTransaction) Trying(hdrs ...base.SipHeader) {
	trying := base.NewResponse(
		tx.origin.SipVersion(),
		100,
		"Trying",
		[]base.SipHeader{},
		"",
		tx.Log(),
	)

	base.CopyHeaders("Via", tx.origin, trying)
	base.CopyHeaders("From", tx.origin, trying)
	base.CopyHeaders("To", tx.origin, trying)
	base.CopyHeaders("Call-Id", tx.origin, trying)
	base.CopyHeaders("CSeq", tx.origin, trying)
	// RFC 3261 - 8.2.6.1
	// Any Timestamp header field present in the request MUST be copied into this 100 (Trying) response.
	// TODO delay?
	base.CopyHeaders("Timestamp", tx.origin, trying)
	// additional custom headers
	for _, h := range hdrs {
		trying.AddHeader(h)
	}

	// change FSM to send provisional response
	tx.lastResp = trying
	tx.fsm.Spin(server_input_user_1xx)
}

func (tx *ServerTransaction) Ok() {

}
