package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"git.sr.ht/~rumpelsepp/helpers"
	"git.sr.ht/~sircmpwn/getopt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
	case "stdio":
		c2 = NewStdioWrapper()
	default:
		c, err := net.Dial("tcp", p.target)
		if err != nil {
			conn.Close()
		}
		c2 = c
	}
	helpers.BidirectCopy(c1, c2)
}

type runtimeOptions struct {
	keepalive  int
	listen     string
	listenPath string
	target     string
	// TODO: make list
	header string
}

func main() {
	opts := runtimeOptions{}
	getopt.StringVar(&opts.header, "H", "", "Specify request header")
	getopt.IntVar(&opts.keepalive, "k", 0, "Set ping interval in seconds")
	getopt.StringVar(&opts.listen, "l", "", "Set listen address")
	getopt.StringVar(&opts.listenPath, "p", "/ws", "Set uri path")
	getopt.StringVar(&opts.target, "t", "-", "Set target to proxy or connect to")
	h := getopt.Bool("h", false, "Show this page and exit")
	if err := getopt.Parse(); err != nil {
		panic(err)
	}

	if *h {
		getopt.Usage()
		os.Exit(0)
	}

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
		var (
			d         = websocket.DefaultDialer
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
			c.SetKeepAlive(time.Duration(opts.keepalive) * time.Second)
		}
		helpers.BidirectCopy(c, s)
	}
}
