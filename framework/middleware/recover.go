package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/memory198/go-gear/framework"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				framework.WriteError(w, framework.ErrInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
