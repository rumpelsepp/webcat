package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/spf13/pflag"
)

type stdioWrapper struct {
	stdin  *os.File
	stdout *os.File
}

func NewStdioWrapper() *stdioWrapper {
	return &stdioWrapper{os.Stdin, os.Stdout}
}

func (w *stdioWrapper) Read(p []byte) (int, error) {
	return w.stdin.Read(p)
}

func (w *stdioWrapper) Write(p []byte) (int, error) {
	return w.stdout.Write(p)
}

func (w *stdioWrapper) Close() error {
	if err := w.stdin.Close(); err != nil {
		return err
	}
	if err := w.stdout.Close(); err != nil {
		return err
	}
	return nil
}

type proxy struct {
	target       string
	pingInterval time.Duration
}

func (p *proxy) handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	var (
		c1 = NewWSTransportWrapper(conn)
		c2 io.ReadWriteCloser
	)
	switch p.target {
	case "-":
		c2 = NewStdioWrapper()
	default:
		c, err := net.Dial("tcp", p.target)
		if err != nil {
			fmt.Println(err)
			return
		}
		c2 = c
	}
	bidirectCopy(c1, c2)
}

type runtimeOptions struct {
	keepalive   int
	listen      string
	listenPath  string
	target      string
	fingerprint string
	// TODO: make list
	header string
}

func main() {
	opts := runtimeOptions{}
	pflag.StringVarP(&opts.header, "header", "H", "", "Specify request header")
	pflag.IntVarP(&opts.keepalive, "keepalive", "k", 0, "Set ping interval in seconds")
	pflag.StringVarP(&opts.fingerprint, "fingerprint", "f", "", "Set SHA-256 fingerprint of certificate")
	pflag.StringVarP(&opts.listen, "listen", "l", "", "Set listen address")
	pflag.StringVarP(&opts.listenPath, "path", "p", "/ws", "Set uri path")
	pflag.StringVarP(&opts.target, "target", "t", "-", "Set target to proxy or connect to")
	pflag.Parse()

	if opts.listen != "" {
		p := proxy{
			target:       opts.target,
			pingInterval: time.Duration(opts.keepalive) * time.Second,
		}
		r := mux.NewRouter()
		r.HandleFunc(opts.listenPath, p.handleWS)
		srv := &http.Server{
			Addr:         opts.listen,
			WriteTimeout: time.Second * 15,
			ReadTimeout:  time.Second * 15,
			IdleTimeout:  time.Second * 60,
			Handler:      r,
		}
		if err := srv.ListenAndServe(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		if opts.target == "-" {
			fmt.Println("error: invalid target")
			os.Exit(1)
		}

		d := websocket.DefaultDialer
		if opts.fingerprint != "" {
			fp, err := hex.DecodeString(opts.fingerprint)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			d.TLSClientConfig = &tls.Config{
				VerifyConnection: func(cs tls.ConnectionState) error {
					digest := sha256.Sum256(cs.PeerCertificates[0].Raw)
					if bytes.Equal(fp, digest[:]) {
						return nil
					}
					return fmt.Errorf("invalid cert: %x; expected %x", fp, digest[:])
				},
			}
		}

		var (
			reqHeader = make(http.Header)
		)
		if opts.header != "" {
			p := strings.SplitN(opts.header, ":", 2)
			reqHeader.Set(p[0], p[1])
		}
		conn, _, err := d.Dial(opts.target, reqHeader)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var (
			// TODO: fix panic
			c = NewWSTransportWrapper(conn)
			s = NewStdioWrapper()
		)
		if opts.keepalive > 0 {
			go c.SetKeepAlive(time.Duration(opts.keepalive) * time.Second)
		}
		bidirectCopy(c, s)
	}
}
