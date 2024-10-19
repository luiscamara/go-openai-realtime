package gorilla

import (
	"context"
	"io"
	"net/http"

	openairt "github.com/WqyJh/go-openai-realtime"
	"github.com/gorilla/websocket"
)

type WebSocketOptions struct {
	ReadLimit int64
	Dialer    *websocket.Dialer
}

type WebSocketDialer struct {
	options WebSocketOptions
}

func NewWebSocketDialer(options WebSocketOptions) *WebSocketDialer {
	if options.Dialer == nil {
		options.Dialer = websocket.DefaultDialer
	}
	return &WebSocketDialer{
		options: options,
	}
}

func (d *WebSocketDialer) Dial(ctx context.Context, url string, header http.Header) (openairt.WebSocketConn, error) {
	conn, resp, err := d.options.Dialer.DialContext(ctx, url, header)
	if resp != nil && resp.Body != nil {
		// The resp.Body is no longer needed after the dial succeeds.
		// When dial fails, the resp.Body contains the original body of the response,
		// which we don't need now.
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	conn.SetReadLimit(d.options.ReadLimit)

	return &WebSocketConn{
		conn:    conn,
		resp:    resp,
		options: d.options,
	}, nil
}

type WebSocketConn struct {
	conn    *websocket.Conn
	resp    *http.Response
	options WebSocketOptions
}

func (c *WebSocketConn) ReadMessage(ctx context.Context) (openairt.MessageType, []byte, error) {
	deadline, ok := ctx.Deadline()
	if ok {
		_ = c.conn.SetReadDeadline(deadline)
	}

	// NextReader would block until the message is read or the connection is closed.
	// It won't be canceled by the ctx before its deadline.
	messageType, r, err := c.conn.NextReader()
	if err != nil {
		// The returned error is Permanent, the future read operations on the same connection will not succeed.
		return 0, nil, openairt.Permanent(err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return 0, nil, err
	}

	switch messageType {
	case websocket.TextMessage:
		return openairt.MessageText, data, nil
	case websocket.BinaryMessage:
		return openairt.MessageBinary, data, nil
	default:
		return 0, nil, openairt.ErrUnsupportedMessageType
	}
}

func (c *WebSocketConn) WriteMessage(ctx context.Context, messageType openairt.MessageType, data []byte) error {
	deadline, ok := ctx.Deadline()
	if ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}

	switch messageType {
	case openairt.MessageText:
		return openairt.Permanent(c.conn.WriteMessage(websocket.TextMessage, data))
	case openairt.MessageBinary:
		return openairt.Permanent(c.conn.WriteMessage(websocket.BinaryMessage, data))
	default:
		return openairt.ErrUnsupportedMessageType
	}
}

func (c *WebSocketConn) Close() error {
	return c.conn.Close()
}

func (c *WebSocketConn) Response() *http.Response {
	return c.resp
}