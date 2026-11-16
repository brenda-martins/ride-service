package httpapi

import (
	"encoding/json"
	"net/http"
)

const problemContentType = "application/problem+json"

// Problem represents an RFC 9457 problem details response body.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code,omitempty"`
	TraceID  string `json:"traceId,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, title, code, detail string) {
	p := Problem{
		Type:    "about:blank",
		Title:   title,
		Status:  status,
		Detail:  detail,
		Code:    code,
		TraceID: traceID(r),
	}
	w.Header().Set("Content-Type", problemContentType+"; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
