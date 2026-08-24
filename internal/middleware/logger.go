package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// RedactingLogger returns a chi Logger middleware that redacts URL path prefixes
// from log output (e.g. to hide webhook secrets embedded in URL segments).
func RedactingLogger(redactPrefixes ...string) func(http.Handler) http.Handler {
	formatter := &redactingFormatter{
		inner:    &chimw.DefaultLogFormatter{Logger: log.New(os.Stdout, "", log.LstdFlags)},
		prefixes: redactPrefixes,
	}
	return chimw.RequestLogger(formatter)
}

type redactingFormatter struct {
	inner    *chimw.DefaultLogFormatter
	prefixes []string
}

func (f *redactingFormatter) NewLogEntry(r *http.Request) chimw.LogEntry {
	for _, prefix := range f.prefixes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			u := *r.URL
			u.Path = prefix + "[redacted]"
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = &u
			r2.RequestURI = prefix + "[redacted]"
			return f.inner.NewLogEntry(r2)
		}
	}
	return f.inner.NewLogEntry(r)
}
