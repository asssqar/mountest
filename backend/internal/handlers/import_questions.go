package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mountest/backend/internal/httpx"
)

type importQuestionsReq struct {
	Text    string `json:"text"`
	Replace bool   `json:"replace"` // true — удалить старые вопросы варианта перед импортом
}

type importQuestionsResp struct {
	Imported int `json:"imported"`
}

func (h *AdminHandler) ImportQuestions(w http.ResponseWriter, r *http.Request) {
	variantID, ok := httpx.ParseUUID(chi.URLParam(r, "id"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "bad id")
		return
	}
	if !h.ensureVariantAccess(w, r, variantID) {
		return
	}

	var req importQuestionsReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	// Ошибки парсинга предназначены для пользователя (формат текста) — оставляем сообщение как есть.
	parsed, err := parseImportPayload(req.Text)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())

	if req.Replace {
		if _, err := tx.Exec(r.Context(),
			`DELETE FROM questions WHERE variant_id=$1`, variantID,
		); err != nil {
			httpx.WriteInternalError(w, r, err)
			return
		}
	}

	var maxPos int
	if err := tx.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(position), 0) FROM questions WHERE variant_id=$1`,
		variantID,
	).Scan(&maxPos); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}

	for i, q := range parsed {
		pos := maxPos + i + 1
		var qid uuid.UUID
		if err := tx.QueryRow(r.Context(),
			`INSERT INTO questions (variant_id, position, text)
			 VALUES ($1, $2, $3) RETURNING id`,
			variantID, pos, strings.TrimSpace(q.Text),
		).Scan(&qid); err != nil {
			httpx.WriteInternalError(w, r, err)
			return
		}
		for j, o := range q.Options {
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO answer_options (question_id, position, text, is_correct)
				 VALUES ($1, $2, $3, $4)`,
				qid, j+1, strings.TrimSpace(o.Text), o.IsCorrect,
			); err != nil {
				httpx.WriteInternalError(w, r, err)
				return
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteInternalError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, importQuestionsResp{Imported: len(parsed)})
}
