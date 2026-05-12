package handlers

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/mountest/backend/internal/auth"
	"github.com/mountest/backend/internal/httpx"
)

// UsersHandler — CRUD над аккаунтами админки. Доступен только superadmin
// (заворачивается middleware'ом RequireSuperadmin на уровне маршрутов).
type UsersHandler struct {
	Pool *pgxpool.Pool
}

type adminUserOut struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(),
		`SELECT id, username, role, created_at FROM admins ORDER BY role DESC, created_at`,
	)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []adminUserOut{}
	for rows.Next() {
		var u adminUserOut
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, u)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type createUserReq struct {
	Email string `json:"email"`
}

func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный email")
		return
	}

	password := randomPassword(14)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось создать пользователя")
		return
	}

	var id uuid.UUID
	err = h.Pool.QueryRow(r.Context(),
		`INSERT INTO admins (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id`,
		email, string(hash), auth.RoleEditor,
	).Scan(&id)
	if err != nil {
		// 23505 — unique_violation на username
		if strings.Contains(err.Error(), "23505") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			httpx.WriteError(w, http.StatusConflict, "Пользователь с таким email уже существует")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":       id,
		"username": email,
		"role":     auth.RoleEditor,
		"password": password, // одноразово отдаём пароль наружу
	})
}

func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	var role string
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT role FROM admins WHERE id=$1`, id,
	).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Пользователь не найден")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if role == auth.RoleSuperadmin {
		httpx.WriteError(w, http.StatusForbidden, "Сброс пароля суперадмина — через переменную окружения ADMIN_PASSWORD")
		return
	}

	password := randomPassword(14)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось сбросить пароль")
		return
	}
	if _, err := h.Pool.Exec(r.Context(),
		`UPDATE admins SET password_hash=$1 WHERE id=$2`, string(hash), id,
	); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":       id,
		"password": password,
	})
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}

	currentID, _ := auth.AdminIDFrom(r.Context())
	if currentID == id.String() {
		httpx.WriteError(w, http.StatusBadRequest, "Нельзя удалить самого себя")
		return
	}

	var role string
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT role FROM admins WHERE id=$1`, id,
	).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "Пользователь не найден")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if role == auth.RoleSuperadmin {
		httpx.WriteError(w, http.StatusForbidden, "Нельзя удалить суперадмина")
		return
	}

	// ON DELETE SET NULL на variants.created_by — данные остаются, владелец обнуляется.
	if _, err := h.Pool.Exec(r.Context(), `DELETE FROM admins WHERE id=$1`, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// randomPassword — без визуально похожих и неоднозначных символов
// (0/O, 1/l/I), чтобы пароли можно было копировать на слух.
func randomPassword(length int) string {
	const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	max := big.NewInt(int64(len(charset)))
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// в проде не должно быть, но fallback на повтор символа лучше чем паника
			buf[i] = charset[0]
			continue
		}
		buf[i] = charset[n.Int64()]
	}
	return string(buf)
}
