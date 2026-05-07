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

// computeAttemptResult пересчитывает балл попытки и собирает экран ошибок.
// Вопрос засчитан только если множества выбранных и правильных опций совпадают.
func computeAttemptResult(ctx context.Context, q querier, attemptID, variantID uuid.UUID) (score, total int, errs []errorEntry, err error) {
	rows, err := q.Query(ctx,
		`SELECT id, position, text FROM questions
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
	}
	var qs []qInfo
	for rows.Next() {
		var qi qInfo
		if err := rows.Scan(&qi.ID, &qi.Position, &qi.Text); err != nil {
			rows.Close()
			return 0, 0, nil, err
		}
		qs = append(qs, qi)
	}
	rows.Close()
	total = len(qs)
	if total == 0 {
		return 0, 0, []errorEntry{}, nil
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
		optsByQ[qid] = append(optsByQ[qid], optionOut{ID: id, Position: pos, Text: text, IsCorrect: isCorrect})
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

	errs = []errorEntry{}
	for _, qi := range qs {
		correctSet := correctByQ[qi.ID]
		selected := selectedByQ[qi.ID]
		selectedSet := make(map[uuid.UUID]struct{}, len(selected))
		for _, s := range selected {
			selectedSet[s] = struct{}{}
		}
		match := len(selectedSet) == len(correctSet)
		if match {
			for k := range correctSet {
				if _, ok := selectedSet[k]; !ok {
					match = false
					break
				}
			}
		}
		if match && len(correctSet) > 0 {
			score++
			continue
		}
		correctIDs := make([]uuid.UUID, 0, len(correctSet))
		for k := range correctSet {
			correctIDs = append(correctIDs, k)
		}
		// чистим is_correct в опциях, чтобы публичный ответ не палил всё (используем correctOptionIds).
		opts := make([]optionOut, 0, len(optsByQ[qi.ID]))
		for _, o := range optsByQ[qi.ID] {
			o.IsCorrect = false
			opts = append(opts, o)
		}
		errs = append(errs, errorEntry{
			QuestionID:        qi.ID,
			QuestionText:      qi.Text,
			Options:           opts,
			SelectedOptionIDs: selected,
			CorrectOptionIDs:  correctIDs,
		})
	}
	return score, total, errs, nil
}
