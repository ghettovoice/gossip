package transaction

import (
	"time"

	"github.com/discoviking/fsm"
	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/timing"
)

// ClientTransaction describes SIP client transaction.
type ClientTransaction struct {
	transaction

	tu           chan *base.Response // Channel to transaction user.
	timer_a_time time.Duration       // Current duration of timer A.
	timer_a      timing.Timer
	timer_b      timing.Timer
	timer_d_time time.Duration // Current duration of timer A.
	timer_d      timing.Timer
}

func NewClientTransaction(req *base.Request, dest string, tm *Manager) *ClientTransaction {
	tx := &ClientTransaction{}
	tx.origin = req
	tx.dest = dest
	tx.tm = tm
	tx.transport = tm.transport
	tx.tu = make(chan *base.Response, 3)
	tx.tu_err = make(chan error, 1)

	return tx
}

// Init performs Client transaction timers initialization and prepares it to send over transport.
func (tx *ClientTransaction) Init() {
	tx.initFSM()
	// RFC 3261 - 17.1.1.2.
	// If an unreliable transport is being used, the client transaction MUST start timer A with a value of T1.
	// If a reliable transport is being used, the client transaction SHOULD NOT
	// start timer A (Timer A controls request retransmissions).
	// Timer A - retransmission
	if !tx.transport.IsReliable() {
		tx.Log().Debugf("client transaction %p, timer_a set to %v", tx, Timer_A)
		tx.timer_a_time = Timer_A
		tx.timer_a = timing.AfterFunc(tx.timer_a_time, func() {
			tx.Log().Debugf("client transaction %p, timer_a fired", tx)
			tx.fsm.Spin(client_input_timer_a)
		})
	}
	// Timer B - timeout
	tx.Log().Debugf("client transaction %p, timer_b set to %v", tx, Timer_B)
	tx.timer_b = timing.AfterFunc(Timer_B, func() {
		tx.Log().Debugf("client transaction %p, timer_b fired", tx)
		tx.fsm.Spin(client_input_timer_b)
	})
	// Timer D is set to 32 seconds for unreliable transports, and 0 seconds otherwise.
	if tx.transport.IsReliable() {
		tx.timer_d_time = 0
	} else {
		tx.timer_d_time = Timer_D
	}
}

// TODO should be refactored to not use tm reference.
func (tx *ClientTransaction) Delete() {
	tx.Log().Debugf("deleting transaction %p from manager %p", tx, tx.tm)
	err := tx.tm.delClientTx(tx)
	if err != nil {
		tx.Log().Warn(err)
		return
	}
}

func (tx *ClientTransaction) Receive(msg base.SipMessage) {
	res, ok := msg.(*base.Response)
	if !ok {
		tx.Log().Errorf("client transaction %p received wrong message %s, response expected", tx, msg.Short())
		return
	}

	tx.lastResp = res

	var input fsm.Input
	switch {
	case res.IsProvisional():
		input = client_input_1xx
	case res.IsSuccess():
		input = client_input_2xx
	default:
		input = client_input_300_plus
	}

	tx.fsm.Spin(input)
}

// Resend the originating request.
//func (tx *ClientTransaction) resend() {
//	tx.Log().Infof("client transaction %p resending request: %v", tx, tx.origin.Short())
//	err := tx.transport.Send(tx.dest, tx.origin)
//	if err != nil {
//		tx.fsm.Spin(client_input_transport_err)
//	}
//}
//
//// Pass up the most recently received response to the TU.
//func (tx *ClientTransaction) passUp() {
//	tx.Log().Infof("client transaction %p passing up response: %v", tx, tx.lastResp.Short())
//	tx.tu <- tx.lastResp
//}
//
//// Send an error to the TU.
//func (tx *ClientTransaction) transportError() {
//	err := "failed to send request"
//	if tx.lastErr != nil {
//		err = tx.lastErr.Error()
//	}
//	tx.Log().Infof("client transaction %p had a transport-level error: %s", tx, err)
//	tx.tu_err <- fmt.Errorf("transport error occurred: %s", err)
//}
//
//// Inform the TU that the transaction timed out.
//func (tx *ClientTransaction) timeoutError() {
//	tx.Log().Infof("client transaction %p timed out", tx)
//	tx.tu_err <- fmt.Errorf("client transaction %p timed out", tx)
//}

// Return the channel we send responses on.
func (tx *ClientTransaction) Responses() <-chan *base.Response {
	return (<-chan *base.Response)(tx.tu)
}

// ack sends an automatic ACK on non 2xx response - RFC 3261 - 17.1.1.3.
func (tx *ClientTransaction) ack() {
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
	cseq, err := tx.origin.CSeq()
	if err != nil {
		tx.Log().Errorf("failed to send ACK request on client transaction %p: %s", tx, err)
		return
	}
	cseq = cseq.Copy().(*base.CSeq)
	cseq.MethodName = base.ACK
	ack.AddHeader(cseq)
	via, err := tx.origin.Via()
	if err != nil {
		tx.Log().Errorf("failed to send ACK request on client transaction %p: %s", tx, err)
		return
	}
	via = via.Copy().(*base.ViaHeader)
	ack.AddHeader(via)
	// Copy headers from response.
	base.CopyHeaders("To", tx.lastResp, ack)

	// Send the ACK.
	err = tx.transport.Send(tx.dest, ack)
	if err != nil {
		tx.Log().Warnf("failed to send ACK request on client transaction %p: %s", tx, err)
		tx.lastErr = err
		tx.fsm.Spin(client_input_transport_err)
	}
}

// Cancel sends CANCEL request - RFC 3261 - 9.
func (tx *ClientTransaction) Cancel() {
	// TODO implement
}
