package peon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"google.golang.org/grpc"
)

func TestPeon(t *testing.T) {
	r := chi.NewRouter()

	r.Get("/foo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hey you")
	})

	s := New(
		OptionAddr(":8080"),
		OptionGRPC(),
		OptionHTTP1(&http.Server{
			Handler: r,
		}),
	)

	if err := s.ListenAndServe(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOptionGraceful(t *testing.T) {
	s := &S{}
	OptionGraceful()(s)

	if s.graceful != true {
		t.Fatal("graceful option should be true, got false")
	}
}

func TestOptionShutdownTimeout(t *testing.T) {
	s := &S{}
	shutdownTimeout := 2 * time.Second
	OptionShutdownTimeout(shutdownTimeout)(s)

	if s.shutdownTimeout != shutdownTimeout {
		t.Fatalf("s.shutdownTimeout should be %v, got %v", shutdownTimeout, s.shutdownTimeout)
	}
}

func TestOptionAddr(t *testing.T) {
	s := &S{}
	addr := ":1234"
	OptionAddr(addr)(s)

	if s.addr != addr {
		t.Fatalf("s.shutdownTimeout should be %v, got %v", addr, s.addr)
	}
}

func TestOptionGRPC(t *testing.T) {
	s := &S{}
	grpcOpt := grpc.ConnectionTimeout(1 * time.Second)
	OptionGRPC(grpcOpt)(s)

	if s.GRPCServer == nil {
		t.Fatal("s.GRPCServer should not be nil")
	}
}

func TestOptionHTTP1(t *testing.T) {
	s := &S{}
	httpSrv := &http.Server{}
	OptionHTTP1(httpSrv)(s)

	if s.HTTP1Server != httpSrv {
		t.Fatal("s.HTTP1 should be set with the http.Server instance passed in via OptionHTTP1")
	}
}

func TestDefaultAddr(t *testing.T) {

	addrWithoutEnv := defaultAddr()
	if addrWithoutEnv != DefaultAddr {
		t.Fatalf("addrWithoutEnv should equal %v, got %v", DefaultAddr, addrWithoutEnv)
	}

	envPortVal := "1234"
	os.Setenv(ENVPort, envPortVal)
	defer os.Setenv(ENVPort, "")

	addrWithEnv := defaultAddr()
	if addrWithEnv != ":"+envPortVal {
		t.Fatalf("addrWithEnv should equal %v, got %v", ":"+envPortVal, addrWithEnv)
	}
}
