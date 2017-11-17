package transaction

import (
	"testing"
	"time"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
)

func TestResendInviteOK(t *testing.T) {
}

func TestInviteOk(t *testing.T) {
	branch := base.GenerateBranch()
	logger := log.WithField("test", t.Name())
	invite, err := request([]string{
		"INVITE sip:bob@example.com SIP/2.0",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + branch,
		"CSeq: 1 INVITE",
		"",
		"",
	}, logger)
	assertNoError(t, err)

	trying, err := response([]string{
		"SIP/2.0 100 Trying",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + branch,
		"CSeq: 1 INVITE",
		"",
		"",
	}, logger)
	assertNoError(t, err)

	ok, err := response([]string{
		"SIP/2.0 200 OK",
		"CSeq: 1 INVITE",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + branch,
		"",
		"",
	}, logger)
	assertNoError(t, err)

	ack, err := request([]string{
		"ACK sip:bob@example.com SIP/2.0",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + base.GenerateBranch(),
		"CSeq: 1 ACK",
		"",
		"",
	}, logger)
	assertNoError(t, err)

	test := transactionTest{
		t:   t,
		log: logger,
		actions: []action{
			&userSend{invite},
			&transportRecv{invite},
			&transportSend{invite},
			&userRecvSrv{invite},
			&transportRecv{trying},
			&wait{time.Second},
			&transportSend{ok},
			&userRecv{ok},
			&wait{time.Second},
			&userSend{ack},
			&transportSend{ack},
			&userRecvSrv{ack},
		}}
	test.Execute()
}

func TestInviteNotOk(t *testing.T) {
	branch := base.GenerateBranch()
	logger := log.WithField("test", t.Name())
	invite, err := request([]string{
		"INVITE sip:bob@example.com SIP/2.0",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + branch,
		"CSeq: 1 INVITE",
		"",
		"",
	}, logger)
	assertNoError(t, err)

	trying, err := response([]string{
		"SIP/2.0 100 Trying",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + branch,
		"CSeq: 1 INVITE",
		"",
		"",
	}, logger)
	assertNoError(t, err)

	ok, err := response([]string{
		"SIP/2.0 200 OK",
		"CSeq: 1 INVITE",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + branch,
		"",
		"",
	}, logger)
	assertNoError(t, err)

	ack, err := request([]string{
		"ACK sip:bob@example.com SIP/2.0",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=" + base.GenerateBranch(),
		"CSeq: 1 ACK",
		"",
		"",
	}, logger)
	assertNoError(t, err)

	test := transactionTest{
		t:   t,
		log: logger,
		actions: []action{
			&userSend{invite},
			&transportRecv{invite},
			&transportSend{invite},
			&userRecvSrv{invite},
			&transportRecv{trying},
			&wait{time.Second},
			&transportSend{ok},
			&userRecv{ok},
			&wait{time.Second},
			&userSend{ack},
			&transportSend{ack},
			&userRecvSrv{ack},
		}}
	test.Execute()
}
