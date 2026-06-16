package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// WriteInternalError логирует полную ошибку с request_id и возвращает клиенту
// безопасное сообщение + requestId, чтобы пользователь мог его указать в баг-репорте.
// НИКОГДА не отдаёт err.Error() наружу — там часто торчат имена таблиц/колонок Postgres.
func WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetReqID(r.Context())
	log.Printf("[req=%s] internal error: %v", reqID, err)
	WriteJSON(w, http.StatusInternalServerError, map[string]string{
		"error":     "Внутренняя ошибка сервера",
		"requestId": reqID,
	})
}

func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func ParseUUID(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// IsUniqueViolation — типизированный детект unique_violation вместо хрупкого
// strings.Contains(err.Error(), "23505").
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
