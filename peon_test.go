//+build unit

package peon

import (
	"net/http"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
)

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

func TestOptionHTTP(t *testing.T) {
	s := &S{}
	httpSrv := &http.Server{}
	OptionHTTP(httpSrv)(s)

	if s.HTTPServer != httpSrv {
		t.Fatal("s.HTTP should be set with the http.Server instance passed in via OptionHTTP")
	}
}

func TestRegisterOnShutdown(t *testing.T) {
	s := &S{}
	funcs := []func(){
		func() {},
		func() {},
		func() {},
	}

	for _, f := range funcs {
		s.RegisterOnShutdown(f)
	}

	if len(s.onShutdown) != len(funcs) {
		t.Fatalf("len(s.onShutdown) should equal %v, got %v", len(funcs), len(s.onShutdown))
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
