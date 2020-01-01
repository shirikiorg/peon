// Package peon offers a server wrapping *http.Server to avoid
// some bikeshedding when making graceful servers
package peon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	// ENVPort is the environment variable key for peon's port
	ENVPort = "PORT"
	// DefaultAddr is the port value that peon's will default to if no
	// environment variable `ENVPort` is set
	DefaultAddr = "8080"
)

// make sure *S implements Server
// var _ Server = (*S)(nil)

// Server is the interface of an http server
type Server interface {
	Close() error
	Shutdown(context.Context) error
	RegisterOnShutdown(func())
	ListenAndServe(context.Context) error
}

// S is an implementation of Server
type S struct {
	GRPCServer      *grpc.Server
	HTTP1Server     *http.Server
	addr            string
	graceful        bool
	shutdownTimeout time.Duration
	signals         []os.Signal
}

// Option is a function type to add options
// to the server
type Option func(s *S)

// New creates a new server with the given options
func New(opts ...Option) *S {
	s := &S{
		signals: []os.Signal{
			os.Interrupt,
			syscall.SIGINT,
			syscall.SIGTERM,
		},
		addr: defaultAddr(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// OptionGraceful makes the server graceful
func OptionGraceful() Option {
	return func(s *S) {
		s.graceful = true
	}
}

// OptionShutdownTimeout sets the shutdown timeout
// for the server
func OptionShutdownTimeout(t time.Duration) Option {
	return func(s *S) {
		s.shutdownTimeout = t
	}
}

// OptionAddr sets the address of the server
func OptionAddr(addr string) Option {
	return func(s *S) {
		s.addr = addr
	}
}

// OptionGRPC adds a GRPC server with the given options
func OptionGRPC(opts ...grpc.ServerOption) Option {
	return func(s *S) {
		s.GRPCServer = grpc.NewServer(opts...)
	}
}

// OptionHTTP1 adds an http server
func OptionHTTP1(srv *http.Server) Option {
	return func(s *S) {
		s.HTTP1Server = srv
	}
}

// ListenAndServe wraps the default http.ListenAndServe
// with graceful shutdown
// According to the underlying S options ListenAndServe will listen
// for incoming connection for http protocol and/or grpc protocol
//
// If both s.GRPCServer & s.HTTP1Server are set the ListenAndServe will
// use the `github.com/soheilhy/cmux` package under the hood in order
// to redirect incoming request according to the http `content-type` header.
func (s *S) ListenAndServe(ctx context.Context) error {
	done := make(chan os.Signal, 1)
	signal.Notify(done, s.signals...)

	// Create the main listener.
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	grpcL := l
	http1L := l
	if s.HTTP1Server != nil && s.GRPCServer != nil {
		// Create a cmux.
		m := cmux.New(l)

		// Match connections in order:
		// First grpc, then HTTP, and otherwise Go RPC/TCP.
		grpcL = m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
		http1L = m.Match(cmux.HTTP1Fast())

		g.Go(func() error {
			return m.Serve()
		})
	}

	if s.HTTP1Server != nil {
		g.Go(func() error {
			fmt.Printf("listening grpc at %s\n", s.addr)
			return s.HTTP1Server.Serve(http1L)
		})
	}
	if s.GRPCServer != nil {
		g.Go(func() error {
			fmt.Printf("listening http at %s\n", s.addr)
			return s.GRPCServer.Serve(grpcL)
		})
	}

	select {
	case <-ctx.Done():
	case <-done:
	}

	return nil
}

// defaultAddr returns a properly formatted addr
func defaultAddr() string {
	p := os.Getenv(ENVPort)
	if p != "" {
		return ":" + p
	}
	return DefaultAddr
}
