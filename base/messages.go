package base

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ghettovoice/gossip/log"
)

// A representation of a SIP method.
// This is syntactic sugar around the string type, so make sure to use
// the Equals method rather than built-in equality, or you'll fall foul of case differences.
// If you're defining your own Method, uppercase is preferred but not compulsory.
type Method string

// Determine if the given method equals some other given method.
// This is syntactic sugar for case insensitive equality checking.
func (method *Method) Equals(other *Method) bool {
	if method != nil && other != nil {
		return strings.EqualFold(string(*method), string(*other))
	} else {
		return method == other
	}
}

// It's nicer to avoid using raw strings to represent methods, so the following standard
// method names are defined here as constants for convenience.
const (
	INVITE    Method = "INVITE"
	ACK       Method = "ACK"
	CANCEL    Method = "CANCEL"
	BYE       Method = "BYE"
	REGISTER  Method = "REGISTER"
	OPTIONS   Method = "OPTIONS"
	SUBSCRIBE Method = "SUBSCRIBE"
	NOTIFY    Method = "NOTIFY"
	REFER     Method = "REFER"
)

// Internal representation of a SIP message - either a Request or a Response.
type SipMessage interface {
	log.WithLocalLogger
	// Yields a flat, string representation of the SIP message suitable for sending out over the wire.
	String() string

	// Adds a header to this message.
	AddHeader(h SipHeader)
	AddFrontHeader(h SipHeader)
	SetHeader(h SipHeader, replace bool)
	SetFrontHeader(h SipHeader, replace bool)

	// Returns a slice of all headers of the given type.
	// If there are no headers of the requested type, returns an empty slice.
	Headers(name string) []SipHeader

	// Return all headers attached to the message, as a slice.
	AllHeaders() []SipHeader

	// Yields a short string representation of the message useful for logging.
	Short() string

	// Remove the specified header from the message.
	RemoveHeader(header SipHeader) error

	SipVersion() string
	SetSipVersion(version string)

	// Get the body of the message, as a string.
	Body() string

	// Set the body of the message.
	SetBody(body string)
	// StartLine returns first line of message.
	StartLine() string
	// Helper getters
	CallId() (*CallId, error)
	Via() (*ViaHeader, error)
	// ViaHop returns first hop from the first Via header.
	ViaHop() (*ViaHop, error)
	Branch() (MaybeString, error)
	From() (*FromHeader, error)
	FromTag() (MaybeString, error)
	To() (*ToHeader, error)
	ToTag() (MaybeString, error)
	CSeq() (*CSeq, error)
}

// A shared type for holding headers and their ordering.
type headers struct {
	// The logical SIP headers attached to this message.
	headers map[string][]SipHeader

	// The order the headers should be displayed in.
	headerOrder []string
}

func newHeaders(hdrs []SipHeader) *headers {
	hs := new(headers)
	hs.headers = make(map[string][]SipHeader)
	hs.headerOrder = make([]string, 0)
	for _, header := range hdrs {
		hs.AddHeader(header)
	}
	return hs
}

func (hs headers) String() string {
	buffer := bytes.Buffer{}
	// Construct each header in turn and add it to the message.
	for typeIdx, name := range hs.headerOrder {
		headers := hs.headers[name]
		for idx, header := range headers {
			buffer.WriteString(header.String())
			if typeIdx < len(hs.headerOrder) || idx < len(headers) {
				buffer.WriteString("\r\n")
			}
		}
	}
	return buffer.String()
}

// Add the given header.
func (hs *headers) AddHeader(h SipHeader) {
	name := strings.ToLower(h.Name())
	if _, ok := hs.headers[name]; ok {
		hs.headers[name] = append(hs.headers[name], h)
	} else {
		hs.headers[name] = []SipHeader{h}
		hs.headerOrder = append(hs.headerOrder, name)
	}
}

// SetHeader works like AddHeader but can drop existing header.
func (hs *headers) SetHeader(h SipHeader, remove bool) {
	name := strings.ToLower(h.Name())
	_, found := hs.headers[name]
	if remove && found {
		delete(hs.headers, name)
		for i, entry := range hs.headerOrder {
			if entry == name {
				hs.headerOrder = append(hs.headerOrder[:i], hs.headerOrder[i+1:]...)
			}
		}
	}

	hs.AddHeader(h)
}

// SetHeader works like AddFrontHeader but can drop existing header.
func (hs *headers) SetFrontHeader(h SipHeader, remove bool) {
	name := strings.ToLower(h.Name())
	_, found := hs.headers[name]
	if remove && found {
		delete(hs.headers, name)
		for i, entry := range hs.headerOrder {
			if entry == name {
				hs.headerOrder = append(hs.headerOrder[:i], hs.headerOrder[i+1:]...)
			}
		}
	}

	hs.AddFrontHeader(h)
}

// AddFrontHeader adds header to the front of header list
// if there is no header has h's name, add h to the tail of all headers
// if there are some headers have h's name, add h to front of the sublist
func (hs *headers) AddFrontHeader(h SipHeader) {
	name := strings.ToLower(h.Name())
	if hdrs, ok := hs.headers[name]; ok {
		newHdrs := make([]SipHeader, 1, len(hdrs)+1)
		newHdrs[0] = h
		hs.headers[name] = append(newHdrs, hdrs...)
	} else {
		hs.headers[name] = []SipHeader{h}
		hs.headerOrder = append(hs.headerOrder, name)
	}
}

// Gets some headers.
func (hs *headers) Headers(name string) []SipHeader {
	name = strings.ToLower(name)
	if hs.headers == nil {
		hs.headers = map[string][]SipHeader{}
		hs.headerOrder = []string{}
	}
	if headers, ok := hs.headers[name]; ok {
		return headers
	} else {
		return []SipHeader{}
	}
}

func (hs *headers) AllHeaders() []SipHeader {
	allHeaders := make([]SipHeader, 0)
	for _, key := range hs.headerOrder {
		allHeaders = append(allHeaders, hs.headers[key]...)
	}

	return allHeaders
}

func (hs *headers) CallId() (*CallId, error) {
	hdrs := hs.Headers("Call-Id")
	if len(hdrs) == 0 {
		return nil, fmt.Errorf("'Call-Id' header not found")
	}
	callId, ok := hdrs[0].(*CallId)
	if !ok {
		return nil, fmt.Errorf("Headers('Call-Id') returned non 'Call-Id' header")
	}
	return callId, nil
}

func (hs *headers) Via() (*ViaHeader, error) {
	hdrs := hs.Headers("Via")
	if len(hdrs) == 0 {
		return nil, fmt.Errorf("'Via' header not found")
	}
	via, ok := hdrs[0].(*ViaHeader)
	if !ok {
		return nil, fmt.Errorf("Headers('Via') returned non 'Via' header")
	}
	return via, nil
}

func (hs *headers) ViaHop() (*ViaHop, error) {
	via, err := hs.Via()
	if err != nil {
		return nil, err
	}
	if len(*via) == 0 {
		return nil, fmt.Errorf("no hops found in the first 'Via' header")
	}
	return (*via)[0], nil
}

func (hs *headers) Branch() (MaybeString, error) {
	via, err := hs.Via()
	if err != nil {
		return nil, err
	}
	branch, ok := (*via)[0].Params.Get("branch")
	if !ok {
		return nil, fmt.Errorf("no 'branch' parameter on top 'Via' header")
	}
	return branch, nil
}

func (hs *headers) From() (*FromHeader, error) {
	hdrs := hs.Headers("From")
	if len(hdrs) == 0 {
		return nil, fmt.Errorf("'From' header not found")
	}
	from, ok := hdrs[0].(*FromHeader)
	if !ok {
		return nil, fmt.Errorf("Headers('From') returned non 'From' header")
	}
	return from, nil
}

func (hs *headers) FromTag() (MaybeString, error) {
	from, err := hs.From()
	if err != nil {
		return nil, err
	}
	tag, ok := from.Params.Get("tag")
	if !ok {
		return nil, fmt.Errorf("no 'tag' parameter on 'From' header")
	}
	return tag, nil
}

func (hs *headers) To() (*ToHeader, error) {
	hdrs := hs.Headers("To")
	if len(hdrs) == 0 {
		return nil, fmt.Errorf("'To' header not found")
	}
	to, ok := hdrs[0].(*ToHeader)
	if !ok {
		return nil, fmt.Errorf("Headers('To') returned non 'To' header")
	}
	return to, nil
}

func (hs *headers) ToTag() (MaybeString, error) {
	to, err := hs.To()
	if err != nil {
		return nil, err
	}
	tag, ok := to.Params.Get("tag")
	if !ok {
		return nil, fmt.Errorf("no 'tag' parameter on 'To' header")
	}
	return tag, nil
}

func (hs *headers) CSeq() (*CSeq, error) {
	hdrs := hs.Headers("CSeq")
	if len(hdrs) == 0 {
		return nil, fmt.Errorf("'CSeq' header not found")
	}
	cseq, ok := hdrs[0].(*CSeq)
	if !ok {
		return nil, fmt.Errorf("Headers('CSeq') returned non 'CSeq' header")
	}
	return cseq, nil
}

func (hs *headers) RemoveHeader(header SipHeader) error {
	errNoMatch := fmt.Errorf(
		"cannot remove header '%s' from message as it is not present",
		header.String(),
	)
	name := strings.ToLower(header.Name())

	headersOfSameType, isMatch := hs.headers[name]
	if !isMatch || len(headersOfSameType) == 0 {
		return errNoMatch
	}

	found := false
	for idx, hdr := range headersOfSameType {
		if hdr == header {
			hs.headers[name] = append(headersOfSameType[:idx], headersOfSameType[idx+1:]...)
			found = true
			break
		}
	}
	if !found {
		return errNoMatch
	}

	if len(hs.headers[name]) == 0 {
		// The header we removed was the only one of its type.
		// Tidy up the header structure by removing the empty list value from the header map,
		// and removing the entry from the headerOrder list.
		delete(hs.headers, name)

		for idx, entry := range hs.headerOrder {
			if entry == name {
				hs.headerOrder = append(hs.headerOrder[:idx], hs.headerOrder[idx+1:]...)
			}
		}
	}

	return nil
}

// Copy all headers of one type from one message to another.
// Appending to any headers that were already there.
func CopyHeaders(name string, from, to SipMessage) {
	name = strings.ToLower(name)
	for _, h := range from.Headers(name) {
		to.AddHeader(h.Copy())
	}
}

// CopyFrontHeaders works like CopyHeaders but adds headers to the front.
func CopyFrontHeaders(name string, from, to SipMessage) {
	name = strings.ToLower(name)
	for _, h := range from.Headers(name) {
		to.AddFrontHeader(h.Copy())
	}
}

// message incorporates generic message methods.
type message struct {
	// A response has headers.
	*headers
	// The version of SIP used in this message, e.g. "SIP/2.0".
	sipVersion string
	// The application data of the message.
	body string
	log  log.Logger
}

func (msg *message) SipVersion() string {
	return msg.sipVersion
}

func (msg *message) SetSipVersion(version string) {
	msg.sipVersion = version
}

func (msg *message) logFields() map[string]interface{} {
	fields := make(map[string]interface{})
	fields["msg-ptr"] = fmt.Sprintf("%p", msg)
	// add cseq
	if cseq, err := msg.CSeq(); err == nil {
		fields["cseq"] = cseq
	}
	// add Call-Id
	if callId, err := msg.CallId(); err == nil {
		fields["call-id"] = string(*callId)
	}
	// add branch
	if branch, err := msg.Branch(); err == nil {
		fields["branch"] = branch
	}
	// add From
	if from, err := msg.From(); err == nil {
		fields["from"] = from
	}
	// add To
	if to, err := msg.To(); err == nil {
		fields["to"] = to
	}

	return fields
}

func (msg *message) Body() string {
	return msg.body
}

func (msg *message) SetBody(body string) {
	msg.body = body
	hdrs := msg.Headers("Content-Length")
	if len(hdrs) == 0 {
		length := ContentLength(len(body))
		msg.AddHeader(length)
	} else {
		hdrs[0] = ContentLength(len(body))
	}
}

func (msg *message) Log() log.Logger {
	return msg.log.WithFields(msg.logFields())
}

// A SIP request (c.f. RFC 3261 section 7.1).
type Request struct {
	message
	// Which method this request is, e.g. an INVITE or a REGISTER.
	Method Method

	// The Request URI. This indicates the user to whom this request is being addressed.
	Recipient Uri
}

func NewRequest(
	method Method,
	recipient Uri,
	sipVersion string,
	headers []SipHeader,
	body string,
	logger log.Logger,
) (request *Request) {
	request = new(Request)
	request.SetSipVersion(sipVersion)
	request.headers = newHeaders(headers)
	request.Method = method
	request.Recipient = recipient
	request.SetBody(body)
	request.log = logger

	return
}

// StartLine returns Request Line - RFC 2361 7.1.
func (request *Request) StartLine() string {
	var buffer bytes.Buffer

	// Every SIP request starts with a Request Line - RFC 2361 7.1.
	buffer.WriteString(
		fmt.Sprintf(
			"%s %s %s",
			(string)(request.Method),
			request.Recipient.String(),
			request.SipVersion(),
		),
	)

	return buffer.String()
}

func (request *Request) Short() string {
	var buffer bytes.Buffer

	buffer.WriteString(request.StartLine())

	cseqs := request.Headers("CSeq")
	if len(cseqs) > 0 {
		buffer.WriteString(fmt.Sprintf(" (%s)", cseqs[0].(*CSeq).String()))
	}

	return buffer.String()
}

func (request *Request) String() string {
	var buffer bytes.Buffer

	// write message start line
	buffer.WriteString(request.StartLine() + "\r\n")
	// Write the headers.
	buffer.WriteString(request.headers.String())
	// If the request has a message body, add it.
	buffer.WriteString("\r\n" + request.Body())

	return buffer.String()
}

func (request *Request) IsInvite() bool {
	return request.Method == INVITE
}

func (request *Request) IsAck() bool {
	return request.Method == ACK
}

// A SIP response object  (c.f. RFC 3261 section 7.2).
type Response struct {
	message
	// The response code, e.g. 200, 401 or 500.
	// This indicates the outcome of the originating request.
	StatusCode uint16

	// The reason string provides additional, human-readable information used to provide
	// clarification or explanation of the status code.
	// This will vary between different SIP UAs, and should not be interpreted by the receiving UA.
	Reason string
}

func NewResponse(
	sipVersion string,
	statusCode uint16,
	reason string,
	headers []SipHeader,
	body string,
	logger log.Logger,
) (response *Response) {
	response = new(Response)
	response.SetSipVersion(sipVersion)
	response.headers = newHeaders(headers)
	response.StatusCode = statusCode
	response.Reason = reason
	response.SetBody(body)
	response.log = logger

	return
}

// StartLine returns Response Status Line - RFC 2361 7.2.
func (response *Response) StartLine() string {
	var buffer bytes.Buffer

	// Every SIP response starts with a Status Line - RFC 2361 7.2.
	buffer.WriteString(
		fmt.Sprintf(
			"%s %d %s",
			response.SipVersion(),
			response.StatusCode,
			response.Reason,
		),
	)

	return buffer.String()
}

func (response *Response) Short() string {
	var buffer bytes.Buffer

	buffer.WriteString(response.StartLine())

	cseqs := response.Headers("CSeq")
	if len(cseqs) > 0 {
		buffer.WriteString(fmt.Sprintf(" (%s)", cseqs[0].(*CSeq).String()))
	}

	return buffer.String()
}

func (response *Response) String() string {
	var buffer bytes.Buffer

	// write message start line
	buffer.WriteString(response.StartLine() + "\r\n")
	// Write the headers.
	buffer.WriteString(response.headers.String())
	// If the request has a message body, add it.
	buffer.WriteString("\r\n" + response.Body())

	return buffer.String()
}

func (response *Response) IsProvisional() bool {
	return response.StatusCode < 200
}

func (response *Response) IsSuccess() bool {
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func (response *Response) IsRedirection() bool {
	return response.StatusCode >= 300 && response.StatusCode < 400
}

func (response *Response) IsClientError() bool {
	return response.StatusCode >= 400 && response.StatusCode < 500
}

func (response *Response) IsServerError() bool {
	return response.StatusCode >= 500 && response.StatusCode < 600
}

func (response *Response) IsGlobalError() bool {
	return response.StatusCode >= 600
}
