package transport

import (
	"net"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/parser"
	"github.com/ghettovoice/gossip/utils"
)

type Tcp struct {
	connTable
	listeningPoints []*net.TCPListener
	parser          *parser.Parser
	output          chan base.SipMessage
	stop            bool
}

func NewTcp(output chan base.SipMessage) (*Tcp, error) {
	tcp := Tcp{output: output}
	tcp.listeningPoints = make([]*net.TCPListener, 0)
	tcp.connTable.Init()
	return &tcp, nil
}

func (tcp *Tcp) Listen(address string) error {
	var err error = nil
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return err
	}

	lp, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	tcp.listeningPoints = append(tcp.listeningPoints, lp)
	go tcp.serve(lp)

	// At this point, err should be nil but let's be defensive.
	return err
}

func (tcp *Tcp) IsStreamed() bool {
	return true
}

func (tcp *Tcp) IsReliable() bool {
	return true
}

func (tcp *Tcp) getConnection(addr string) (*connection, error) {
	conn := tcp.connTable.GetConn(addr)

	if conn == nil {
		logger := log.WithField("conn-tag", utils.RandStr(4, "conn-"))

		logger.Debugf("no stored connection for address %s; generate a new one", addr)
		raddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}

		baseConn, err := net.DialTCP("tcp", nil, raddr)
		if err != nil {
			return nil, err
		}

		conn = NewConn(baseConn, tcp.output, logger)
	} else {
		conn = tcp.connTable.GetConn(addr)
	}

	tcp.connTable.Notify(addr, conn)
	return conn, nil
}

func (tcp *Tcp) Send(addr string, msg base.SipMessage) error {
	msg.Log().Infof("sending message to %v: %v", addr, msg.Short())
	msg.Log().Debugf("sending message:\r\n%v", msg.String())

	conn, err := tcp.getConnection(addr)
	if err != nil {
		return err
	}

	err = conn.Send(msg)
	return err
}

func (tcp *Tcp) serve(listeningPoint *net.TCPListener) {
	log.Infof("begin serving TCP on address %s", listeningPoint.Addr().String())

	iter := func(listeningPoint *net.TCPListener) bool {
		logger := log.WithField("conn-tag", utils.RandStr(4, "conn-"))
		baseConn, err := listeningPoint.Accept()
		if err != nil {
			logger.Errorf(
				"failed to accept TCP conn on address %s: %s",
				listeningPoint.Addr().String(),
				err.Error(),
			)
			return true
		}

		conn := NewConn(baseConn, tcp.output, logger)
		logger.Debugf(
			"accepted new TCP conn %p from %s on address %s",
			&conn,
			conn.baseConn.RemoteAddr(),
			conn.baseConn.LocalAddr(),
		)
		tcp.connTable.Notify(baseConn.RemoteAddr().String(), conn)

		return true
	}
	for {
		if !iter(listeningPoint) {
			break
		}
	}
}

func (tcp *Tcp) Stop() {
	tcp.connTable.Stop()
	tcp.stop = true
	for _, lp := range tcp.listeningPoints {
		lp.Close()
	}
}
