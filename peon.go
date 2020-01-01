// Package peon offers a server wrapping *http.Server to avoid
// some bikeshedding when making graceful servers
package peon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/shirikiorg/wait"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	// ENVPort is the environment variable key for peon's port
	ENVPort = "PORT"
	// DefaultAddr is the port value that peon's will default to if no
	// environment variable `ENVPort` is set
	DefaultAddr = ":8080"
)

// make sure *S implements Server
var _ Server = (*S)(nil)

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
	HTTPServer      *http.Server
	addr            string
	graceful        bool
	shutdownTimeout time.Duration
	signals         []os.Signal
	mu              sync.Mutex
	onShutdown      []func()
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

// OptionHTTP adds an http server
func OptionHTTP(srv *http.Server) Option {
	return func(s *S) {
		s.HTTPServer = srv
	}
}

// ListenAndServe wraps the default http.ListenAndServe
// with graceful shutdown
// According to the underlying S options ListenAndServe will listen
// for incoming connection for http protocol and/or grpc protocol
//
// If both s.GRPCServer & s.HTTPServer are set the ListenAndServe will
// use the `github.com/soheilhy/cmux` package under the hood in order
// to redirect incoming request according to the http `content-type` header.
func (s *S) ListenAndServe(ctx context.Context) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, s.signals...)

	// Create the main listener.
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	grpcL := l
	http1L := l
	if s.HTTPServer != nil && s.GRPCServer != nil {
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

	if s.HTTPServer != nil {
		g.Go(func() error {
			fmt.Printf("listening http at %s\n", s.addr)
			return s.HTTPServer.Serve(http1L)
		})
	}
	if s.GRPCServer != nil {
		g.Go(func() error {
			fmt.Printf("listening grpc at %s\n", s.addr)
			return s.GRPCServer.Serve(grpcL)
		})
	}

	var cancel func()

	select {
	case <-ctx.Done():
		// here we don't wrap the top context because it's
		// already done
		ctx, cancel = context.WithTimeout(context.Background(), s.shutdownTimeout)
	case <-interrupt:
		ctx, cancel = context.WithTimeout(ctx, s.shutdownTimeout)
	}

	defer cancel()

	return s.Shutdown(ctx)
}

// defaultAddr returns a properly formatted addr
func defaultAddr() string {
	p := os.Getenv(ENVPort)
	if p != "" {
		return ":" + p
	}
	return DefaultAddr
}

// Shutdown gracefully shuts down the any underlying servers without
// interrupting any active connections.
// When shutdown starts it runs all the registered functions
// in parallel. Shutdown waits for all operations to be completed before
// returning.
func (s *S) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		err error
		wg  wait.Group
	)

	if s.HTTPServer != nil {
		wg.Add(1)
		go func() {
			err = s.HTTPServer.Shutdown(ctx)
			wg.Done()
		}()
	}

	if s.GRPCServer != nil {
		wg.Add(1)
		go func() {
			s.GRPCServer.GracefulStop()
			wg.Done()
		}()
	}

	wg.Add(len(s.onShutdown))
	for _, f := range s.onShutdown {
		go func(f func()) {
			defer wg.Done()
			f()
		}(f)
	}

	if waitErr := wg.WaitWithContext(ctx); waitErr != nil {
		return waitErr
	}
	return err
}

// RegisterOnShutdown registers a function to call on Shutdown.
func (s *S) RegisterOnShutdown(f func()) {
	s.mu.Lock()
	s.onShutdown = append(s.onShutdown, f)
	s.mu.Unlock()
}

// Close immediately closes all active on the underlying servers
func (s *S) Close() error {
	if s.GRPCServer != nil && s.HTTPServer != nil {
		var (
			wg  sync.WaitGroup
			err error
		)

		wg.Add(2)
		go func() {
			defer wg.Done()
			err = s.HTTPServer.Close()
		}()

		go func() {
			defer wg.Done()
			s.GRPCServer.Stop()
		}()

		wg.Wait()
		return err
	}

	if s.GRPCServer != nil {
		s.GRPCServer.Stop()
		return nil
	}

	if s.HTTPServer != nil {
		return s.HTTPServer.Close()
	}

	return errors.New("nothing to close")
}
