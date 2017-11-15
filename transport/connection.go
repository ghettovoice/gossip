package transport

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/parser"
)

type connection struct {
	baseConn       net.Conn
	isStreamed     bool
	parser         parser.Parser
	parsedMessages chan base.SipMessage
	parserErrors   chan error
	output         chan base.SipMessage
	log            log.Logger
}

func NewConn(baseConn net.Conn, output chan base.SipMessage, logger log.Logger) *connection {
	var isStreamed bool
	switch baseConn.(type) {
	case *net.UDPConn:
		isStreamed = false
	case *net.TCPConn:
		isStreamed = true
	case *tls.Conn:
		isStreamed = true
	default:
		logger.Errorf(
			"conn object %v is not a known connection type. "+
				"Assume it's a streamed protocol, but this may cause messages to be rejected",
			baseConn,
		)
	}
	connection := connection{baseConn: baseConn, isStreamed: isStreamed, log: logger}

	connection.parsedMessages = make(chan base.SipMessage)
	connection.parserErrors = make(chan error)
	connection.output = output
	connection.parser = parser.NewParser(
		connection.parsedMessages,
		connection.parserErrors,
		connection.isStreamed,
		logger,
	)

	go connection.read()
	go connection.pipeOutput()

	return &connection
}

func (connection *connection) Log() log.Logger {
	return connection.log
}

func (connection *connection) Send(msg base.SipMessage) (err error) {
	connection.Log().Debugf("sending message over connection %p: %s", connection, msg.Short())
	msgData := msg.String()
	n, err := connection.baseConn.Write([]byte(msgData))

	if err != nil {
		return
	}

	if n != len(msgData) {
		return fmt.Errorf("not all data was sent when dispatching '%s' to %s",
			msg.Short(), connection.baseConn.RemoteAddr())
	}

	return
}

func (connection *connection) Close() error {
	connection.Log().Debugf("connection for address %s expired, will be removed", connection.baseConn.RemoteAddr())
	connection.parser.Stop()
	return connection.baseConn.Close()
}

func (connection *connection) read() {
	buffer := make([]byte, c_BUFSIZE)
	for {
		connection.Log().Debugf("connection %p waiting for new data on sock", connection)
		num, err := connection.baseConn.Read(buffer)
		if err != nil {
			// If connections are broken, just let them drop.
			connection.Log().Debugf(
				"lost connection to %s on %s",
				connection.baseConn.RemoteAddr().String(),
				connection.baseConn.LocalAddr().String(),
			)
			return
		}

		connection.Log().Debugf("connection %p received %d bytes", connection, num)
		pkt := append([]byte(nil), buffer[:num]...)
		connection.parser.Write(pkt)
	}
}

func (connection *connection) pipeOutput() {
	for {
		select {
		case message, ok := <-connection.parsedMessages:
			if ok {
				connection.Log().Debugf(
					"connection %p from %s to %s received message over the wire: %s",
					connection,
					connection.baseConn.RemoteAddr(),
					connection.baseConn.LocalAddr(),
					message.Short(),
				)
				connection.output <- message
			} else {
				break
			}
		case err, ok := <-connection.parserErrors:
			if ok {
				// The parser has hit a terminal error. We need to restart it.
				connection.Log().Warnf("failed to parse SIP message: %s", err.Error())
				connection.parser = parser.NewParser(
					connection.parsedMessages,
					connection.parserErrors,
					connection.isStreamed,
					connection.Log(),
				)
			} else {
				break
			}
		}
	}

	connection.Log().Infof(
		"parser stopped in ConnWrapper %v (local addr %s; remote addr %s); stopping listening",
		connection,
		connection.baseConn.LocalAddr(),
		connection.baseConn.RemoteAddr(),
	)
}
