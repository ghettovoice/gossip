package transaction

import (
	"testing"
	"time"
)

func TestResendInviteOK(t *testing.T) {
}

func TestReceiveInvite(t *testing.T) {
	invite, err := request([]string{
		"INVITE sip:joe@bloggs.com SIP/2.0",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=z9hG4bK776asdhds",
		"CSeq: 1 INVITE",
		"",
		"",
	})
	assertNoError(t, err)

	trying, err := response([]string{
		"SIP/2.0 100 Trying",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=z9hG4bK776asdhds",
		"CSeq: 1 INVITE",
		"",
		"",
	})
	assertNoError(t, err)

	ok, err := response([]string{
		"SIP/2.0 200 OK",
		"CSeq: 1 INVITE",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=z9hG4bK776asdhds",
		"",
		"",
	})
	assertNoError(t, err)

	ack, err := request([]string{
		"ACK sip:joe@bloggs.com SIP/2.0",
		"Via: SIP/2.0/UDP " + c_CLIENT + ";branch=z9hG4bKagjbewssw",
		"CSeq: 1 ACK",
		"",
		"",
	})
	assertNoError(t, err)

	test := transactionTest{t: t,
		actions: []action{
			&transportSend{invite}, // client sends INVITE
			&transportRecv{trying}, // server sends 100 trying
			&wait{time.Second},
			&transportSend{ok}, // server sends 200 OK
			&transportRecv{ok}, // client gets 100 trying
			&wait{time.Second},
			&transportSend{ack}, // client sends ACK
			&transportRecv{ack}, // server gets ACK
		}}
	test.Execute()
}
