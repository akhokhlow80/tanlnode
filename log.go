package main

import (
	"fmt"
	"log"
	"net/http"
)

func logReqPrintf(r *http.Request, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s %s from %s: %s", r.Method, r.URL, r.RemoteAddr, msg)
}
