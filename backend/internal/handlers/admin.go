package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/mountest/backend/internal/auth"
	"github.com/mountest/backend/internal/httpx"
)

type AdminHandler struct {
	Pool *pgxpool.Pool
	Auth *auth.Service
}

// ---- auth ----

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Введите логин и пароль")
		return
	}

	var (
		id   uuid.UUID
		hash string
		role string
	)
	err := h.Pool.QueryRow(r.Context(),
		`SELECT id, password_hash, role FROM admins WHERE username=$1`, req.Username,
	).Scan(&id, &hash, &role)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}

	tok, exp, err := h.Auth.Issue(id, role)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось выдать токен")
		return
	}
	h.Auth.SetCookie(w, tok, exp)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"username": req.Username,
		"role":     role,
	})
}

func (h *AdminHandler) Logout(w http.ResponseWriter, _ *http.Request) {
	h.Auth.ClearCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.AdminIDFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var username, role string
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT username, role FROM admins WHERE id=$1`, id,
	).Scan(&username, &role); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"username": username,
		"role":     role,
	})
}

// ---- ownership helpers ----

// currentAdminUUID — парсит UUID авторизованного пользователя из контекста.
// Если по какой-то причине нет — возвращает uuid.Nil (хендлер должен реагировать).
func currentAdminUUID(ctx context.Context) uuid.UUID {
	idStr, ok := auth.AdminIDFrom(ctx)
	if !ok {
		return uuid.Nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// ensureVariantAccess: суперадмин — всегда ОК; editor — только если он владелец.
// Возвращает (httpStatus, errMsg). 0 — доступ разрешён.
func (h *AdminHandler) ensureVariantAccess(ctx context.Context, variantID uuid.UUID) (int, string) {
	if auth.IsSuperadmin(ctx) {
		return 0, ""
	}
	adminID := currentAdminUUID(ctx)
	if adminID == uuid.Nil {
		return http.StatusUnauthorized, "unauthorized"
	}
	var owner *uuid.UUID
	err := h.Pool.QueryRow(ctx,
		`SELECT created_by FROM variants WHERE id=$1`, variantID,
	).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound, "Вариант не найден"
	}
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	if owner == nil || *owner != adminID {
		return http.StatusForbidden, "Нет доступа к этому варианту"
	}
	return 0, ""
}

// ensureQuestionAccess — то же, но через JOIN questions→variants.
func (h *AdminHandler) ensureQuestionAccess(ctx context.Context, questionID uuid.UUID) (int, string) {
	if auth.IsSuperadmin(ctx) {
		return 0, ""
	}
	adminID := currentAdminUUID(ctx)
	if adminID == uuid.Nil {
		return http.StatusUnauthorized, "unauthorized"
	}
	var owner *uuid.UUID
	err := h.Pool.QueryRow(ctx,
		`SELECT v.created_by FROM questions q
		 JOIN variants v ON v.id = q.variant_id
		 WHERE q.id=$1`, questionID,
	).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound, "Вопрос не найден"
	}
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	if owner == nil || *owner != adminID {
		return http.StatusForbidden, "Нет доступа к этому вопросу"
	}
	return 0, ""
}

// ---- subjects ----

type subjectReq struct {
	Name string `json:"name"`
}

func (h *AdminHandler) ListSubjects(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(),
		`SELECT s.id, s.name,
		        (SELECT COUNT(*) FROM variants v WHERE v.subject_id = s.id) AS variants_count
		 FROM subjects s ORDER BY s.name`)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID            uuid.UUID `json:"id"`
		Name          string    `json:"name"`
		VariantsCount int       `json:"variantsCount"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Name, &it.VariantsCount); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, it)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) CreateSubject(w http.ResponseWriter, r *http.Request) {
	var req subjectReq
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Укажите название")
		return
	}
	var id uuid.UUID
	err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO subjects (name) VALUES ($1) RETURNING id`,
		strings.TrimSpace(req.Name),
	).Scan(&id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "name": req.Name})
}

func (h *AdminHandler) UpdateSubject(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	var req subjectReq
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Укажите название")
		return
	}
	tag, err := h.Pool.Exec(r.Context(),
		`UPDATE subjects SET name=$1 WHERE id=$2`,
		strings.TrimSpace(req.Name), id,
	)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "name": req.Name})
}

func (h *AdminHandler) DeleteSubject(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if _, err := h.Pool.Exec(r.Context(), `DELETE FROM subjects WHERE id=$1`, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- variants ----

type variantReq struct {
	SubjectID       uuid.UUID `json:"subjectId"`
	Title           string    `json:"title"`
	DurationMinutes int       `json:"durationMinutes"`
}

func (h *AdminHandler) ListVariants(w http.ResponseWriter, r *http.Request) {
	q := `SELECT v.id, v.subject_id, v.title, v.duration_minutes, v.created_by,
	             (SELECT COUNT(*) FROM questions qq WHERE qq.variant_id = v.id) AS questions_count
	      FROM variants v`
	args := []any{}
	conds := []string{}

	// Editor видит только свои варианты.
	if !auth.IsSuperadmin(r.Context()) {
		adminID := currentAdminUUID(r.Context())
		if adminID == uuid.Nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		conds = append(conds, "v.created_by = $1")
		args = append(args, adminID)
	}

	if subj := r.URL.Query().Get("subjectId"); subj != "" {
		id, ok := httpx.ParseUUID(subj)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "bad subjectId")
			return
		}
		placeholder := "$" + strconv.Itoa(len(args)+1)
		conds = append(conds, "v.subject_id = "+placeholder)
		args = append(args, id)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY v.title"

	rows, err := h.Pool.Query(r.Context(), q, args...)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID              uuid.UUID  `json:"id"`
		SubjectID       uuid.UUID  `json:"subjectId"`
		Title           string     `json:"title"`
		DurationMinutes int        `json:"durationMinutes"`
		QuestionsCount  int        `json:"questionsCount"`
		CreatedBy       *uuid.UUID `json:"createdBy,omitempty"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.SubjectID, &it.Title, &it.DurationMinutes, &it.CreatedBy, &it.QuestionsCount); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, it)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) CreateVariant(w http.ResponseWriter, r *http.Request) {
	var req variantReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" || req.SubjectID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "Укажите предмет и название")
		return
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60
	}
	adminID := currentAdminUUID(r.Context())
	if adminID == uuid.Nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var id uuid.UUID
	err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO variants (subject_id, title, duration_minutes, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		req.SubjectID, strings.TrimSpace(req.Title), req.DurationMinutes, adminID,
	).Scan(&id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *AdminHandler) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if status, msg := h.ensureVariantAccess(r.Context(), id); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	var req variantReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" || req.SubjectID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "Укажите предмет и название")
		return
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60
	}
	tag, err := h.Pool.Exec(r.Context(),
		`UPDATE variants SET subject_id=$1, title=$2, duration_minutes=$3 WHERE id=$4`,
		req.SubjectID, strings.TrimSpace(req.Title), req.DurationMinutes, id,
	)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *AdminHandler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if status, msg := h.ensureVariantAccess(r.Context(), id); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	if _, err := h.Pool.Exec(r.Context(), `DELETE FROM variants WHERE id=$1`, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- questions (вместе с опциями для редактора) ----

type optionInput struct {
	ID        *uuid.UUID `json:"id,omitempty"`
	Text      string     `json:"text"`
	IsCorrect bool       `json:"isCorrect"`
}

type questionReq struct {
	VariantID uuid.UUID     `json:"variantId"`
	Position  int           `json:"position"`
	Text      string        `json:"text"`
	Options   []optionInput `json:"options"`
}

type optionOut struct {
	ID        uuid.UUID `json:"id"`
	Text      string    `json:"text"`
	IsCorrect bool      `json:"isCorrect"`
	Position  int       `json:"position"`
}

type questionOut struct {
	ID        uuid.UUID   `json:"id"`
	VariantID uuid.UUID   `json:"variantId"`
	Position  int         `json:"position"`
	Text      string      `json:"text"`
	Options   []optionOut `json:"options"`
}

func (h *AdminHandler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	variantStr := r.URL.Query().Get("variantId")
	if variantStr == "" {
		httpx.WriteError(w, http.StatusBadRequest, "variantId обязателен")
		return
	}
	vid, ok := httpx.ParseUUID(variantStr)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad variantId")
		return
	}
	if status, msg := h.ensureVariantAccess(r.Context(), vid); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	out, err := loadQuestionsWithOptions(r.Context(), h.Pool, vid, true)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) GetQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if status, msg := h.ensureQuestionAccess(r.Context(), id); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	q, err := loadQuestion(r.Context(), h.Pool, id, true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, q)
}

func (h *AdminHandler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	var req questionReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.VariantID == uuid.Nil || strings.TrimSpace(req.Text) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Укажите вариант и текст вопроса")
		return
	}
	if status, msg := h.ensureVariantAccess(r.Context(), req.VariantID); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	if len(req.Options) < 2 {
		httpx.WriteError(w, http.StatusBadRequest, "Нужно минимум 2 варианта ответа")
		return
	}
	hasCorrect := false
	for _, o := range req.Options {
		if strings.TrimSpace(o.Text) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "Все варианты должны иметь текст")
			return
		}
		if o.IsCorrect {
			hasCorrect = true
		}
	}
	if !hasCorrect {
		httpx.WriteError(w, http.StatusBadRequest, "Отметьте хотя бы один правильный ответ")
		return
	}

	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	if req.Position <= 0 {
		var maxPos int
		if err := tx.QueryRow(r.Context(),
			`SELECT COALESCE(MAX(position), 0) FROM questions WHERE variant_id=$1`,
			req.VariantID,
		).Scan(&maxPos); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		req.Position = maxPos + 1
	}

	var qid uuid.UUID
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO questions (variant_id, position, text)
		 VALUES ($1, $2, $3) RETURNING id`,
		req.VariantID, req.Position, strings.TrimSpace(req.Text),
	).Scan(&qid); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i, o := range req.Options {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO answer_options (question_id, position, text, is_correct)
			 VALUES ($1, $2, $3, $4)`,
			qid, i+1, strings.TrimSpace(o.Text), o.IsCorrect,
		); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": qid})
}

func (h *AdminHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if status, msg := h.ensureQuestionAccess(r.Context(), id); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	var req questionReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Текст обязателен")
		return
	}
	if len(req.Options) < 2 {
		httpx.WriteError(w, http.StatusBadRequest, "Нужно минимум 2 варианта ответа")
		return
	}
	hasCorrect := false
	for _, o := range req.Options {
		if strings.TrimSpace(o.Text) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "Все варианты должны иметь текст")
			return
		}
		if o.IsCorrect {
			hasCorrect = true
		}
	}
	if !hasCorrect {
		httpx.WriteError(w, http.StatusBadRequest, "Отметьте хотя бы один правильный ответ")
		return
	}

	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	tag, err := tx.Exec(r.Context(),
		`UPDATE questions SET position=$1, text=$2 WHERE id=$3`,
		req.Position, strings.TrimSpace(req.Text), id,
	)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM answer_options WHERE question_id=$1`, id,
	); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i, o := range req.Options {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO answer_options (question_id, position, text, is_correct)
			 VALUES ($1, $2, $3, $4)`,
			id, i+1, strings.TrimSpace(o.Text), o.IsCorrect,
		); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *AdminHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if status, msg := h.ensureQuestionAccess(r.Context(), id); status != 0 {
		httpx.WriteError(w, status, msg)
		return
	}
	if _, err := h.Pool.Exec(r.Context(), `DELETE FROM questions WHERE id=$1`, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

