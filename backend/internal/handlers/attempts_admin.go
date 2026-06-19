package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mountest/backend/internal/auth"
	"github.com/mountest/backend/internal/httpx"
)

// История попыток для админки.
// Editor видит только попытки по своим вариантам (через variants.created_by).
// Superadmin видит все.

type adminAttemptGuest struct {
	ID        *uuid.UUID `json:"id,omitempty"`
	FirstName string     `json:"firstName"`
	LastName  string     `json:"lastName"`
}

type adminAttemptVariant struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
	Topic *string   `json:"topic,omitempty"`
}

type adminAttemptSubject struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type adminAttemptRow struct {
	ID         uuid.UUID            `json:"id"`
	Score      *int                 `json:"score,omitempty"`
	Total      *int                 `json:"total,omitempty"`
	StartedAt  time.Time            `json:"startedAt"`
	FinishedAt *time.Time           `json:"finishedAt,omitempty"`
	Guest      adminAttemptGuest    `json:"guest"`
	Variant    adminAttemptVariant  `json:"variant"`
	Subject    adminAttemptSubject  `json:"subject"`
}

type adminAttemptsPage struct {
	Items  []adminAttemptRow `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

const (
	defaultAttemptsLimit = 50
	maxAttemptsLimit     = 200
)

// ListAttempts — GET /api/admin/attempts?variantId=&status=&limit=&offset=
// status: "" | "finished" | "active"
func (h *AdminHandler) ListAttempts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, err := parseClampedInt(q.Get("limit"), defaultAttemptsLimit, 1, maxAttemptsLimit)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad limit")
		return
	}
	offset, err := parseClampedInt(q.Get("offset"), 0, 0, 1_000_000)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad offset")
		return
	}

	conds := []string{}
	args := []any{}

	// Editor видит только попытки по своим вариантам.
	if !auth.IsSuperadmin(r.Context()) {
		adminID := currentAdminUUID(r.Context())
		if adminID == uuid.Nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		conds = append(conds, "v.created_by = $"+strconv.Itoa(len(args)+1))
		args = append(args, adminID)
	}

	if vidStr := q.Get("variantId"); vidStr != "" {
		vid, ok := httpx.ParseUUID(vidStr)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "bad variantId")
			return
		}
		conds = append(conds, "a.variant_id = $"+strconv.Itoa(len(args)+1))
		args = append(args, vid)
	}

	switch q.Get("status") {
	case "finished":
		conds = append(conds, "a.finished_at IS NOT NULL")
	case "active":
		conds = append(conds, "a.finished_at IS NULL")
	case "", "all":
		// без фильтра
	default:
		httpx.WriteError(w, http.StatusBadRequest, "bad status")
		return
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	// Считаем total отдельным запросом — с теми же условиями, чтобы пагинация была честной.
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM attempts a
		JOIN variants v ON v.id = a.variant_id
		%s`, where)
	var total int
	if err := h.Pool.QueryRow(r.Context(), countSQL, args...).Scan(&total); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}

	// Пагинационные параметры идут последними плейсхолдерами.
	listSQL := fmt.Sprintf(`
		SELECT a.id, a.score, a.total, a.started_at, a.finished_at,
		       g.id, g.first_name, g.last_name,
		       v.id, v.title, v.topic,
		       s.id, s.name
		FROM attempts a
		JOIN variants v ON v.id = a.variant_id
		JOIN subjects s ON s.id = v.subject_id
		LEFT JOIN guest_sessions g ON g.id = a.guest_session_id
		%s
		ORDER BY a.started_at DESC
		LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	args = append(args, limit, offset)

	rows, err := h.Pool.Query(r.Context(), listSQL, args...)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	defer rows.Close()

	items := []adminAttemptRow{}
	for rows.Next() {
		var (
			it                       adminAttemptRow
			guestID                  *uuid.UUID
			gFirst, gLast            *string
		)
		if err := rows.Scan(
			&it.ID, &it.Score, &it.Total, &it.StartedAt, &it.FinishedAt,
			&guestID, &gFirst, &gLast,
			&it.Variant.ID, &it.Variant.Title, &it.Variant.Topic,
			&it.Subject.ID, &it.Subject.Name,
		); err != nil {
			httpx.WriteInternalError(w, r, err)
			return
		}
		it.Guest = adminAttemptGuest{ID: guestID}
		if gFirst != nil {
			it.Guest.FirstName = *gFirst
		}
		if gLast != nil {
			it.Guest.LastName = *gLast
		}
		items = append(items, it)
	}

	httpx.WriteJSON(w, http.StatusOK, adminAttemptsPage{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// DeleteAttempt — DELETE /api/admin/attempts/{id}
// Superadmin удаляет любую попытку, editor — только по своим вариантам.
// attempt_answers удаляются каскадно (ON DELETE CASCADE в схеме).
func (h *AdminHandler) DeleteAttempt(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}

	var sql string
	var args []any

	if auth.IsSuperadmin(r.Context()) {
		sql = `DELETE FROM attempts WHERE id=$1`
		args = []any{id}
	} else {
		adminID := currentAdminUUID(r.Context())
		if adminID == uuid.Nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		// Editor может удалять только попытки по своим вариантам.
		sql = `DELETE FROM attempts a USING variants v WHERE a.id=$1 AND a.variant_id=v.id AND v.created_by=$2`
		args = []any{id, adminID}
	}

	tag, err := h.Pool.Exec(r.Context(), sql, args...)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseClampedInt(raw string, def, min, max int) (int, error) {
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n < min {
		return min, nil
	}
	if n > max {
		return max, nil
	}
	return n, nil
}
