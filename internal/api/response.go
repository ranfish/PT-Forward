package api

import (
	"encoding/json"
	"net/http"

	"github.com/ranfish/pt-forward/internal/audit"
	"github.com/ranfish/pt-forward/internal/auth"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Detail  string      `json:"detail,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func Error(w http.ResponseWriter, httpStatus int, code int, message string) {
	writeJSON(w, httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

func ErrorWithDetail(w http.ResponseWriter, httpStatus int, code int, message string, detail string) {
	writeJSON(w, httpStatus, Response{
		Code:    code,
		Message: message,
		Detail:  detail,
	})
}

func actorFromRequest(r *http.Request) string {
	if uid, ok := r.Context().Value(auth.CtxKeyUserID).(string); ok && uid != "" {
		return uid
	}
	return "system"
}

func auditLog(r *http.Request, module, action, targetType, targetID, detail, result string) {
	audit.Log(actorFromRequest(r), module, action, targetType, targetID, detail, result)
}
