package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mountest/backend/internal/httpx"
)

type PublicHandler struct {
	Pool *pgxpool.Pool
	// AttemptSecret подписывает токены попытки. Выводится из JWT_SECRET в main.go
	// через DeriveAttemptSecret — отдельной переменной окружения не нужно.
	AttemptSecret []byte
}

// ---- guest sessions ----

type guestReq struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (h *PublicHandler) CreateGuest(w http.ResponseWriter, r *http.Request) {
	var req guestReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	first := strings.TrimSpace(req.FirstName)
	last := strings.TrimSpace(req.LastName)
	if first == "" || last == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Введите имя и фамилию")
		return
	}
	var id uuid.UUID
	if err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO guest_sessions (first_name, last_name)
		 VALUES ($1, $2) RETURNING id`,
		first, last,
	).Scan(&id); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":        id,
		"firstName": first,
		"lastName":  last,
	})
}

func (h *PublicHandler) GetGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	var first, last string
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT first_name, last_name FROM guest_sessions WHERE id=$1`, id,
	).Scan(&first, &last); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":        id,
		"firstName": first,
		"lastName":  last,
	})
}

// ---- subjects / variants (public, без is_correct) ----

func (h *PublicHandler) ListSubjects(w http.ResponseWriter, r *http.Request) {
	// Ученикам показываем только предметы, у которых есть хотя бы один ОПУБЛИКОВАННЫЙ вариант.
	rows, err := h.Pool.Query(r.Context(),
		`SELECT s.id, s.name,
		        (SELECT COUNT(*) FROM variants v WHERE v.subject_id = s.id AND v.is_published) AS variants_count
		 FROM subjects s
		 WHERE EXISTS (SELECT 1 FROM variants v WHERE v.subject_id = s.id AND v.is_published)
		 ORDER BY s.name`)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
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
			httpx.WriteInternalError(w, r, err)
			return
		}
		out = append(out, it)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *PublicHandler) ListSubjectVariants(w http.ResponseWriter, r *http.Request) {
	subjID, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	rows, err := h.Pool.Query(r.Context(),
		`SELECT v.id, v.title, v.topic, v.duration_minutes,
		        (SELECT COUNT(*) FROM questions q WHERE q.variant_id = v.id) AS questions_count
		 FROM variants v WHERE v.subject_id=$1 AND v.is_published
		 ORDER BY v.title`,
		subjID,
	)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID              uuid.UUID `json:"id"`
		Title           string    `json:"title"`
		Topic           *string   `json:"topic,omitempty"`
		DurationMinutes int       `json:"durationMinutes"`
		QuestionsCount  int       `json:"questionsCount"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Title, &it.Topic, &it.DurationMinutes, &it.QuestionsCount); err != nil {
			httpx.WriteInternalError(w, r, err)
			return
		}
		out = append(out, it)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ---- attempts ----

type startAttemptReq struct {
	VariantID      uuid.UUID `json:"variantId"`
	GuestSessionID uuid.UUID `json:"guestSessionId"`
}

type attemptOut struct {
	ID              uuid.UUID           `json:"id"`
	VariantID       uuid.UUID           `json:"variantId"`
	VariantTitle    string              `json:"variantTitle"`
	SubjectName     string              `json:"subjectName"`
	DurationMinutes int                 `json:"durationMinutes"`
	StartedAt       time.Time           `json:"startedAt"`
	FinishedAt      *time.Time          `json:"finishedAt,omitempty"`
	Questions       []questionOut       `json:"questions"`
	Answers         map[string][]string `json:"answers"`
	Guest           *guestPublic        `json:"guest,omitempty"`
	// AttemptToken возвращается ТОЛЬКО при создании попытки. На всех остальных
	// ручках клиент сам передаёт его в заголовке X-Attempt-Token.
	AttemptToken string `json:"attemptToken,omitempty"`
}

type guestPublic struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
}

func (h *PublicHandler) StartAttempt(w http.ResponseWriter, r *http.Request) {
	var req startAttemptReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if req.VariantID == uuid.Nil || req.GuestSessionID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "Нужны variantId и guestSessionId")
		return
	}

	// проверка ссылочной целостности заранее, чтобы вернуть понятную ошибку
	var ok bool
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM guest_sessions WHERE id=$1)`, req.GuestSessionID,
	).Scan(&ok); err != nil || !ok {
		httpx.WriteError(w, http.StatusBadRequest, "Гостевая сессия не найдена")
		return
	}
	// Опубликован И существует — иначе ученик мог бы стартовать по прямой ссылке скрытый вариант.
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM variants WHERE id=$1 AND is_published)`, req.VariantID,
	).Scan(&ok); err != nil || !ok {
		httpx.WriteError(w, http.StatusBadRequest, "Вариант недоступен")
		return
	}

	var id uuid.UUID
	if err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO attempts (guest_session_id, variant_id) VALUES ($1, $2) RETURNING id`,
		req.GuestSessionID, req.VariantID,
	).Scan(&id); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	out, err := h.loadAttempt(r.Context(), id, false)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	// Токен выдаётся ТОЛЬКО при создании попытки. Дальше клиент шлёт его в X-Attempt-Token.
	out.AttemptToken = attemptToken(h.AttemptSecret, id)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *PublicHandler) GetAttempt(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if !h.requireAttemptAuth(w, r, id) {
		return
	}
	h.maybeAutoFinish(r.Context(), id)
	out, err := h.loadAttempt(r.Context(), id, false)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		httpx.WriteInternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type saveAnswerReq struct {
	QuestionID        uuid.UUID   `json:"questionId"`
	SelectedOptionIDs []uuid.UUID `json:"selectedOptionIds"`
}

func (h *PublicHandler) SaveAnswer(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if !h.requireAttemptAuth(w, r, id) {
		return
	}
	var req saveAnswerReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if req.QuestionID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "Нужен questionId")
		return
	}

	var finished *time.Time
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT finished_at FROM attempts WHERE id=$1`, id,
	).Scan(&finished); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Попытка не найдена")
		return
	}
	if finished != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Попытка уже завершена")
		return
	}

	if req.SelectedOptionIDs == nil {
		req.SelectedOptionIDs = []uuid.UUID{}
	}
	if _, err := h.Pool.Exec(r.Context(),
		`INSERT INTO attempt_answers (attempt_id, question_id, selected_option_ids, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (attempt_id, question_id)
		 DO UPDATE SET selected_option_ids=EXCLUDED.selected_option_ids, updated_at=now()`,
		id, req.QuestionID, req.SelectedOptionIDs,
	); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// reviewEntry — один вопрос на экране результата. Возвращается всегда, по всем
// вопросам варианта, не только по ошибкам. Status позволяет фронту красить и фильтровать.
type reviewEntry struct {
	QuestionID        uuid.UUID   `json:"questionId"`
	Position          int         `json:"position"`
	QuestionText      string      `json:"questionText"`
	QuestionImageURL  *string     `json:"questionImageUrl,omitempty"`
	Options           []optionOut `json:"options"`
	SelectedOptionIDs []uuid.UUID `json:"selectedOptionIds"`
	CorrectOptionIDs  []uuid.UUID `json:"correctOptionIds"`
	// Status: "correct" | "incorrect" | "unanswered".
	Status string `json:"status"`
}

type resultOut struct {
	AttemptID  uuid.UUID     `json:"attemptId"`
	Score      int           `json:"score"`
	Total      int           `json:"total"`
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt time.Time     `json:"finishedAt"`
	Review     []reviewEntry `json:"review"`
	Guest      *guestPublic  `json:"guest,omitempty"`
}

// FinishAttempt считает результат, фиксирует попытку и возвращает экран результата.
func (h *PublicHandler) FinishAttempt(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if !h.requireAttemptAuth(w, r, id) {
		return
	}

	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())

	var (
		variantID uuid.UUID
		started   time.Time
		finished  *time.Time
		guestID   uuid.UUID
	)
	if err := tx.QueryRow(r.Context(),
		`SELECT variant_id, started_at, finished_at, guest_session_id
		 FROM attempts WHERE id=$1 FOR UPDATE`, id,
	).Scan(&variantID, &started, &finished, &guestID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Попытка не найдена")
		return
	}

	score, total, review, err := computeAttemptReview(r.Context(), tx, id, variantID)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}

	now := time.Now()
	if finished == nil {
		if _, err := tx.Exec(r.Context(),
			`UPDATE attempts SET finished_at=$1, score=$2, total=$3 WHERE id=$4`,
			now, score, total, id,
		); err != nil {
			httpx.WriteInternalError(w, r, err)
			return
		}
		finished = &now
	}

	var first, last string
	_ = tx.QueryRow(r.Context(),
		`SELECT first_name, last_name FROM guest_sessions WHERE id=$1`, guestID,
	).Scan(&first, &last)

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resultOut{
		AttemptID:  id,
		Score:      score,
		Total:      total,
		StartedAt:  started,
		FinishedAt: *finished,
		Review:     review,
		Guest:      &guestPublic{ID: guestID, FirstName: first, LastName: last},
	})
}

func (h *PublicHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if !h.requireAttemptAuth(w, r, id) {
		return
	}
	var (
		variantID uuid.UUID
		started   time.Time
		finished  *time.Time
		score     *int
		total     *int
		guestID   uuid.UUID
	)
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT variant_id, started_at, finished_at, score, total, guest_session_id
		 FROM attempts WHERE id=$1`, id,
	).Scan(&variantID, &started, &finished, &score, &total, &guestID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Попытка не найдена")
		return
	}
	if finished == nil {
		httpx.WriteError(w, http.StatusBadRequest, "Попытка ещё не завершена")
		return
	}
	_, _, review, err := computeAttemptReview(r.Context(), h.Pool, id, variantID)
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	var first, last string
	_ = h.Pool.QueryRow(r.Context(),
		`SELECT first_name, last_name FROM guest_sessions WHERE id=$1`, guestID,
	).Scan(&first, &last)

	out := resultOut{
		AttemptID:  id,
		StartedAt:  started,
		FinishedAt: *finished,
		Review:     review,
		Guest:      &guestPublic{ID: guestID, FirstName: first, LastName: last},
	}
	if score != nil {
		out.Score = *score
	}
	if total != nil {
		out.Total = *total
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *PublicHandler) loadAttempt(ctx context.Context, attemptID uuid.UUID, includeCorrect bool) (*attemptOut, error) {
	var (
		variantID uuid.UUID
		started   time.Time
		finished  *time.Time
		guestID   uuid.UUID
	)
	if err := h.Pool.QueryRow(ctx,
		`SELECT variant_id, started_at, finished_at, guest_session_id
		 FROM attempts WHERE id=$1`, attemptID,
	).Scan(&variantID, &started, &finished, &guestID); err != nil {
		return nil, err
	}
	var (
		variantTitle    string
		durationMinutes int
		subjectName     string
	)
	if err := h.Pool.QueryRow(ctx,
		`SELECT v.title, v.duration_minutes, s.name
		 FROM variants v JOIN subjects s ON s.id = v.subject_id
		 WHERE v.id=$1`, variantID,
	).Scan(&variantTitle, &durationMinutes, &subjectName); err != nil {
		return nil, err
	}
	questions, err := loadQuestionsWithOptions(ctx, h.Pool, variantID, includeCorrect)
	if err != nil {
		return nil, err
	}

	answers := map[string][]string{}
	rows, err := h.Pool.Query(ctx,
		`SELECT question_id, selected_option_ids FROM attempt_answers WHERE attempt_id=$1`,
		attemptID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var qid uuid.UUID
		var sel []uuid.UUID
		if err := rows.Scan(&qid, &sel); err != nil {
			return nil, err
		}
		strs := make([]string, 0, len(sel))
		for _, s := range sel {
			strs = append(strs, s.String())
		}
		answers[qid.String()] = strs
	}

	var first, last string
	_ = h.Pool.QueryRow(ctx,
		`SELECT first_name, last_name FROM guest_sessions WHERE id=$1`, guestID,
	).Scan(&first, &last)

	return &attemptOut{
		ID:              attemptID,
		VariantID:       variantID,
		VariantTitle:    variantTitle,
		SubjectName:     subjectName,
		DurationMinutes: durationMinutes,
		StartedAt:       started,
		FinishedAt:      finished,
		Questions:       questions,
		Answers:         answers,
		Guest:           &guestPublic{ID: guestID, FirstName: first, LastName: last},
	}, nil
}
