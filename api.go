package main

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"
)

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func respondJSON[T any](w http.ResponseWriter, code int, result T) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)

	var err error
	if reflect.TypeOf(result).Kind() == reflect.Slice && reflect.ValueOf(result).IsNil() {
		err = json.NewEncoder(w).Encode([]struct{}{})
	} else {
		err = json.NewEncoder(w).Encode(result)
	}
	if err != nil {
		log.Printf("Error writing json: %s", err)
	}
}

func respondError(w http.ResponseWriter, code int, msg string) {
	respondJSON(w, code, APIError{Code: code, Message: msg})
}

func internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	respondError(w, http.StatusInternalServerError, "Internal server erro")
	logReqPrintf(r, "Internal server error: %s", err)
}
