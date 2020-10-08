package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type wrapper struct {
	conn *websocket.Conn
}

func (w *wrapper) Write(p []byte) (int, error) {
	wr, err := w.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(wr, bytes.NewReader(p))
	if err != nil {
		return 0, err
	}
	if err := wr.Close(); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (w *wrapper) Read(p []byte) (int, error) {
	msgType, r, err := w.conn.NextReader()
	if err != nil {
		return 0, err
	}
	if msgType != websocket.BinaryMessage {
		return 0, fmt.Errorf("unexpected message type")
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return 0, err
	}
	return copy(p, buf.Bytes()), nil
}

type WSTransportWrapper struct {
	Conn      *websocket.Conn
	wrap      *wrapper
	bufReader *bufio.Reader
}

func NewWSTransportWrapper(conn *websocket.Conn) *WSTransportWrapper {
	conn.SetPingHandler(nil)
	conn.SetPongHandler(nil)
	wrap := &wrapper{conn}
	return &WSTransportWrapper{
		Conn:      conn,
		wrap:      wrap,
		bufReader: bufio.NewReader(wrap),
	}
}

func (t *WSTransportWrapper) SetKeepAlive(ti time.Duration) {
	go func() {
		for {
			d := time.Now().Add(ti)
			if err := t.Conn.WriteControl(websocket.PingMessage, nil, d); err != nil {
				return
			}
			time.Sleep(ti)
		}
	}()
}

func (t *WSTransportWrapper) Read(p []byte) (int, error) {
	return t.bufReader.Read(p)
}

func (t *WSTransportWrapper) Write(p []byte) (int, error) {
	return t.wrap.Write(p)
}

func (t *WSTransportWrapper) Close() error {
	return t.Conn.Close()
}

func (t *WSTransportWrapper) LocalAddr() net.Addr {
	return t.Conn.LocalAddr()
}

func (t *WSTransportWrapper) SetDeadline(ti time.Time) error {
	return nil
}

func (t *WSTransportWrapper) SetReadDeadline(ti time.Time) error {
	return t.Conn.SetReadDeadline(ti)
}

func (t *WSTransportWrapper) SetWriteDeadline(ti time.Time) error {
	return t.Conn.SetWriteDeadline(ti)
}

func (t *WSTransportWrapper) RemoteAddr() net.Addr {
	return t.Conn.RemoteAddr()
}
