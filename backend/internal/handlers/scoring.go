package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// querier — общий интерфейс для *pgxpool.Pool и pgx.Tx.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Статусы для review-экрана. Совпадают с TS-литералом на фронте.
const (
	reviewStatusCorrect    = "correct"
	reviewStatusIncorrect  = "incorrect"
	reviewStatusUnanswered = "unanswered"
)

// computeAttemptReview пересчитывает балл попытки и собирает полный обзор:
// все вопросы в порядке, с выбранными опциями, правильными опциями и статусом.
// Вопрос засчитан только если множества выбранных и правильных опций совпадают.
func computeAttemptReview(ctx context.Context, q querier, attemptID, variantID uuid.UUID) (score, total int, review []reviewEntry, err error) {
	rows, err := q.Query(ctx,
		`SELECT id, position, text, image_url FROM questions
		 WHERE variant_id=$1 ORDER BY position, created_at`,
		variantID,
	)
	if err != nil {
		return 0, 0, nil, err
	}
	type qInfo struct {
		ID       uuid.UUID
		Position int
		Text     string
		ImageURL *string
	}
	var qs []qInfo
	for rows.Next() {
		var qi qInfo
		if err := rows.Scan(&qi.ID, &qi.Position, &qi.Text, &qi.ImageURL); err != nil {
			rows.Close()
			return 0, 0, nil, err
		}
		qs = append(qs, qi)
	}
	rows.Close()
	total = len(qs)
	if total == 0 {
		return 0, 0, []reviewEntry{}, nil
	}

	ids := make([]uuid.UUID, 0, len(qs))
	for _, qi := range qs {
		ids = append(ids, qi.ID)
	}

	optsByQ := map[uuid.UUID][]optionOut{}
	correctByQ := map[uuid.UUID]map[uuid.UUID]struct{}{}
	optRows, err := q.Query(ctx,
		`SELECT id, question_id, position, text, is_correct
		 FROM answer_options WHERE question_id = ANY($1)
		 ORDER BY position, created_at`,
		ids,
	)
	if err != nil {
		return 0, 0, nil, err
	}
	for optRows.Next() {
		var (
			id        uuid.UUID
			qid       uuid.UUID
			pos       int
			text      string
			isCorrect bool
		)
		if err := optRows.Scan(&id, &qid, &pos, &text, &isCorrect); err != nil {
			optRows.Close()
			return 0, 0, nil, err
		}
		// is_correct в опции не отдаём — фронт пользуется correctOptionIds.
		optsByQ[qid] = append(optsByQ[qid], optionOut{ID: id, Position: pos, Text: text, IsCorrect: false})
		if isCorrect {
			if correctByQ[qid] == nil {
				correctByQ[qid] = map[uuid.UUID]struct{}{}
			}
			correctByQ[qid][id] = struct{}{}
		}
	}
	optRows.Close()

	selectedByQ := map[uuid.UUID][]uuid.UUID{}
	ansRows, err := q.Query(ctx,
		`SELECT question_id, selected_option_ids FROM attempt_answers WHERE attempt_id=$1`,
		attemptID,
	)
	if err != nil {
		return 0, 0, nil, err
	}
	for ansRows.Next() {
		var qid uuid.UUID
		var sel []uuid.UUID
		if err := ansRows.Scan(&qid, &sel); err != nil {
			ansRows.Close()
			return 0, 0, nil, err
		}
		selectedByQ[qid] = sel
	}
	ansRows.Close()

	review = make([]reviewEntry, 0, len(qs))
	for _, qi := range qs {
		correctSet := correctByQ[qi.ID]
		selected := selectedByQ[qi.ID]
		selectedSet := make(map[uuid.UUID]struct{}, len(selected))
		for _, s := range selected {
			selectedSet[s] = struct{}{}
		}

		match := len(selectedSet) == len(correctSet) && len(correctSet) > 0
		if match {
			for k := range correctSet {
				if _, ok := selectedSet[k]; !ok {
					match = false
					break
				}
			}
		}

		var status string
		switch {
		case match:
			status = reviewStatusCorrect
			score++
		case len(selected) == 0:
			status = reviewStatusUnanswered
		default:
			status = reviewStatusIncorrect
		}

		correctIDs := make([]uuid.UUID, 0, len(correctSet))
		for k := range correctSet {
			correctIDs = append(correctIDs, k)
		}
		// selected должен быть не-nil слайсом, иначе JSON выйдет null.
		if selected == nil {
			selected = []uuid.UUID{}
		}

		review = append(review, reviewEntry{
			QuestionID:        qi.ID,
			Position:          qi.Position,
			QuestionText:      qi.Text,
			QuestionImageURL:  qi.ImageURL,
			Options:           optsByQ[qi.ID],
			SelectedOptionIDs: selected,
			CorrectOptionIDs:  correctIDs,
			Status:            status,
		})
	}
	return score, total, review, nil
}
