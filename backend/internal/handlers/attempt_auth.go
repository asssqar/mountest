package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/google/uuid"

	"github.com/mountest/backend/internal/httpx"
)

// DeriveAttemptSecret выводит секрет для подписи токенов попытки из JWT_SECRET
// через домен-сепарацию. Один источник энтропии, без отдельного env var.
func DeriveAttemptSecret(jwtSecret string) []byte {
	h := sha256.New()
	h.Write([]byte("mountest-attempt-v1:"))
	h.Write([]byte(jwtSecret))
	return h.Sum(nil)
}

// attemptToken — детерминированный HMAC от UUID попытки.
// Знание токена эквивалентно «я знаю секрет сервера и id попытки» — то есть
// получить его можно только с легитимного ответа на POST /attempts.
func attemptToken(secret []byte, id uuid.UUID) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// requireAttemptAuth: проверяет заголовок X-Attempt-Token. Если не совпадает,
// записывает 401 и возвращает false. Для всех ручек /attempts/{id}/*.
func (h *PublicHandler) requireAttemptAuth(w http.ResponseWriter, r *http.Request, id uuid.UUID) bool {
	got := r.Header.Get("X-Attempt-Token")
	if got == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизованы для этой попытки")
		return false
	}
	expected := attemptToken(h.AttemptSecret, id)
	// constant-time, чтобы не было side-channel на длину/префикс.
	if !hmac.Equal([]byte(got), []byte(expected)) {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизованы для этой попытки")
		return false
	}
	return true
}
