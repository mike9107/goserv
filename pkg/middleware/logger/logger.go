package logger

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	utilhttp "github.com/cmgsj/goserve/pkg/util/http"
)

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := utilhttp.NewResponseRecorder(w)

		start := time.Now()

		defer func() {
			delta := time.Since(start)
			slog.Info(
				fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				"address", r.RemoteAddr,
				"status", http.StatusText(recorder.StatusCode()),
				"duration", delta,
			)
		}()

		next.ServeHTTP(recorder, r)
	})
}
