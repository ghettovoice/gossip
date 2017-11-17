package transaction

import (
	"github.com/discoviking/fsm"
	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/timing"
)

// SIP Client Transaction FSM
// Implements the behaviour described in RFC 3261 section 17.1

// FSM States
const (
	client_state_calling = iota
	client_state_proceeding
	client_state_completed
	client_state_terminated
)

// FSM Inputs
const (
	client_input_1xx fsm.Input = iota
	client_input_2xx
	client_input_300_plus
	client_input_timer_a
	client_input_timer_b
	client_input_timer_d
	client_input_transport_err
	client_input_delete
)

// Initialises the correct kind of FSM based on request method.
func (tx *ClientTransaction) initFSM() {
	if tx.origin.Method == base.INVITE {
		tx.initInviteFSM()
	} else {
		tx.initNonInviteFSM()
	}
}

func (tx *ClientTransaction) initInviteFSM() {
	tx.Log().Debugf("initialising INVITE client transaction %p FSM", tx)

	// Define Actions
	// Resend the request.
	act_resend := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_resend", tx)
		tx.timer_a_time *= 2
		tx.timer_a.Reset(tx.timer_a_time)
		tx.resend()
		return fsm.NO_INPUT
	}

	// Just pass up the latest response.
	act_passup := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_passup", tx)
		tx.passUp()
		return fsm.NO_INPUT
	}

	// Handle 300+ responses.
	// Pass up response and send ACK, start timer D.
	act_300_plus := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_300_plus", tx)
		tx.passUp()
		tx.ack()
		if tx.timer_d != nil {
			tx.timer_d.Stop()
		}
		tx.timer_d = timing.AfterFunc(tx.timer_d_time, func() {
			tx.fsm.Spin(client_input_timer_d)
		})
		return fsm.NO_INPUT
	}

	// Send an ACK.
	act_ack := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_ack", tx)
		tx.ack()
		return fsm.NO_INPUT
	}

	// Send up transport failure error.
	act_trans_err := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_trans_err", tx)
		tx.transportError()
		return client_input_delete
	}

	// Send up timeout error.
	act_timeout := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_timeout", tx)
		// todo send 408 to TU?
		tx.timeoutError()
		return client_input_delete
	}

	// Pass up the response and delete the transaction.
	act_passup_delete := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_passup_delete", tx)
		tx.passUp()
		return client_input_delete
	}

	// Just delete the transaction.
	act_delete := func() fsm.Input {
		tx.Log().Debugf("INVITE client transaction %p, act_delete", tx)
		tx.Delete()
		return fsm.NO_INPUT
	}

	// Define States

	// Calling
	client_state_def_calling := fsm.State{
		Index: client_state_calling,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:           {client_state_proceeding, act_passup},
			client_input_2xx:           {client_state_terminated, act_passup_delete},
			client_input_300_plus:      {client_state_completed, act_300_plus},
			client_input_timer_a:       {client_state_calling, act_resend},
			client_input_timer_b:       {client_state_terminated, act_timeout},
			client_input_transport_err: {client_state_terminated, act_trans_err},
		},
	}

	// Proceeding
	client_state_def_proceeding := fsm.State{
		Index: client_state_proceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:      {client_state_proceeding, act_passup},
			client_input_2xx:      {client_state_terminated, act_passup_delete},
			client_input_300_plus: {client_state_completed, act_300_plus},
			client_input_timer_a:  {client_state_proceeding, fsm.NO_ACTION},
			client_input_timer_b:  {client_state_proceeding, fsm.NO_ACTION},
		},
	}

	// Completed
	client_state_def_completed := fsm.State{
		Index: client_state_completed,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:           {client_state_completed, fsm.NO_ACTION},
			client_input_2xx:           {client_state_completed, fsm.NO_ACTION},
			client_input_300_plus:      {client_state_completed, act_ack},
			client_input_transport_err: {client_state_terminated, act_trans_err},
			client_input_timer_a:       {client_state_completed, fsm.NO_ACTION},
			client_input_timer_b:       {client_state_completed, fsm.NO_ACTION},
			client_input_timer_d:       {client_state_terminated, act_delete},
		},
	}

	// Terminated
	client_state_def_terminated := fsm.State{
		Index: client_state_terminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:      {client_state_terminated, fsm.NO_ACTION},
			client_input_2xx:      {client_state_terminated, fsm.NO_ACTION},
			client_input_300_plus: {client_state_terminated, fsm.NO_ACTION},
			client_input_timer_a:  {client_state_terminated, fsm.NO_ACTION},
			client_input_timer_b:  {client_state_terminated, fsm.NO_ACTION},
			client_input_timer_d:  {client_state_terminated, fsm.NO_ACTION},
			client_input_delete:   {client_state_terminated, act_delete},
		},
	}

	fsm_, err := fsm.Define(
		client_state_def_calling,
		client_state_def_proceeding,
		client_state_def_completed,
		client_state_def_terminated,
	)

	if err != nil {
		tx.Log().Errorf("failure to define INVITE client transaction %p fsm: %s", tx, err.Error())
	}

	tx.fsm = fsm_
}

func (tx *ClientTransaction) initNonInviteFSM() {
	tx.Log().Debugf("initialising non-INVITE client transaction %p FSM", tx)

	// Define Actions

	// Resend the request.
	act_resend := func() fsm.Input {
		tx.Log().Debugf("non-INVITE client transaction %p, act_resend", tx)
		tx.timer_a_time *= 2
		// For non-INVITE, cap timer A at T2 seconds.
		if tx.timer_a_time > T2 {
			tx.timer_a_time = T2
		}
		tx.timer_a.Reset(tx.timer_a_time)
		tx.resend()
		return fsm.NO_INPUT
	}

	// Just pass up the latest response.
	act_passup := func() fsm.Input {
		tx.Log().Debugf("non-INVITE client transaction %p, act_passup", tx)
		tx.passUp()
		return fsm.NO_INPUT
	}

	// Handle a final response.
	act_final := func() fsm.Input {
		tx.Log().Debugf("non-INVITE client transaction %p, act_final", tx)
		tx.passUp()
		if tx.timer_d != nil {
			tx.timer_d.Stop()
		}
		tx.timer_d = timing.AfterFunc(tx.timer_d_time, func() {
			tx.fsm.Spin(client_input_timer_d)
		})
		return fsm.NO_INPUT
	}

	// Send up transport failure error.
	act_trans_err := func() fsm.Input {
		tx.Log().Debugf("non-INVITE client transaction %p, act_trans_err", tx)
		tx.transportError()
		return client_input_delete
	}

	// Send up timeout error.
	act_timeout := func() fsm.Input {
		tx.Log().Debugf("non-INVITE client transaction %p, act_timeout", tx)
		tx.timeoutError()
		return client_input_delete
	}

	// Just delete the transaction.
	act_delete := func() fsm.Input {
		tx.Log().Debugf("non-INVITE client transaction %p, act_delete", tx)
		tx.Delete()
		return fsm.NO_INPUT
	}

	// Define States

	// "Trying"
	client_state_def_calling := fsm.State{
		Index: client_state_calling,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:           {client_state_proceeding, act_passup},
			client_input_2xx:           {client_state_completed, act_final},
			client_input_300_plus:      {client_state_completed, act_final},
			client_input_timer_a:       {client_state_calling, act_resend},
			client_input_timer_b:       {client_state_terminated, act_timeout},
			client_input_transport_err: {client_state_terminated, act_trans_err},
		},
	}

	// Proceeding
	client_state_def_proceeding := fsm.State{
		Index: client_state_proceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:           {client_state_proceeding, act_passup},
			client_input_2xx:           {client_state_completed, act_final},
			client_input_300_plus:      {client_state_completed, act_final},
			client_input_timer_a:       {client_state_proceeding, act_resend},
			client_input_timer_b:       {client_state_terminated, act_timeout},
			client_input_transport_err: {client_state_terminated, act_trans_err},
		},
	}

	// Completed
	client_state_def_completed := fsm.State{
		Index: client_state_completed,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:      {client_state_completed, fsm.NO_ACTION},
			client_input_2xx:      {client_state_completed, fsm.NO_ACTION},
			client_input_300_plus: {client_state_completed, fsm.NO_ACTION},
			client_input_timer_d:  {client_state_terminated, act_delete},
			client_input_timer_a:  {client_state_completed, fsm.NO_ACTION},
			client_input_timer_b:  {client_state_completed, fsm.NO_ACTION},
		},
	}

	// Terminated
	client_state_def_terminated := fsm.State{
		Index: client_state_terminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			client_input_1xx:      {client_state_terminated, fsm.NO_ACTION},
			client_input_2xx:      {client_state_terminated, fsm.NO_ACTION},
			client_input_300_plus: {client_state_terminated, fsm.NO_ACTION},
			client_input_timer_a:  {client_state_terminated, fsm.NO_ACTION},
			client_input_timer_b:  {client_state_terminated, fsm.NO_ACTION},
			client_input_timer_d:  {client_state_terminated, fsm.NO_ACTION},
			client_input_delete:   {client_state_terminated, act_delete},
		},
	}

	fsm_, err := fsm.Define(
		client_state_def_calling,
		client_state_def_proceeding,
		client_state_def_completed,
		client_state_def_terminated,
	)

	if err != nil {
		tx.Log().Errorf("failure to define INVITE client transaction %p fsm: %s", tx, err.Error())
	}

	tx.fsm = fsm_
}
