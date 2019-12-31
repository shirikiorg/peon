package peon

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi"
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
