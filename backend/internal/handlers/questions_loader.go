package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// loadQuestionsWithOptions возвращает вопросы варианта со всеми опциями.
// includeCorrect=true возвращает признак is_correct (для админки/расчёта результата),
// иначе всегда false (для прохождения теста).
func loadQuestionsWithOptions(ctx context.Context, pool *pgxpool.Pool, variantID uuid.UUID, includeCorrect bool) ([]questionOut, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, position, text, image_url FROM questions
		 WHERE variant_id=$1 ORDER BY position, created_at`,
		variantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []questionOut{}
	idx := map[uuid.UUID]int{}
	for rows.Next() {
		var q questionOut
		q.VariantID = variantID
		if err := rows.Scan(&q.ID, &q.Position, &q.Text, &q.ImageURL); err != nil {
			return nil, err
		}
		q.Options = []optionOut{}
		idx[q.ID] = len(out)
		out = append(out, q)
	}
	if len(out) == 0 {
		return out, nil
	}

	ids := make([]uuid.UUID, 0, len(out))
	for _, q := range out {
		ids = append(ids, q.ID)
	}

	optRows, err := pool.Query(ctx,
		`SELECT id, question_id, position, text, is_correct
		 FROM answer_options WHERE question_id = ANY($1)
		 ORDER BY position, created_at`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer optRows.Close()
	for optRows.Next() {
		var (
			id        uuid.UUID
			qid       uuid.UUID
			pos       int
			text      string
			isCorrect bool
		)
		if err := optRows.Scan(&id, &qid, &pos, &text, &isCorrect); err != nil {
			return nil, err
		}
		opt := optionOut{ID: id, Position: pos, Text: text, IsCorrect: isCorrect}
		if !includeCorrect {
			opt.IsCorrect = false
		}
		i := idx[qid]
		out[i].Options = append(out[i].Options, opt)
	}
	return out, nil
}

func loadQuestion(ctx context.Context, pool *pgxpool.Pool, questionID uuid.UUID, includeCorrect bool) (*questionOut, error) {
	var q questionOut
	if err := pool.QueryRow(ctx,
		`SELECT id, variant_id, position, text, image_url FROM questions WHERE id=$1`,
		questionID,
	).Scan(&q.ID, &q.VariantID, &q.Position, &q.Text, &q.ImageURL); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT id, position, text, is_correct FROM answer_options
		 WHERE question_id=$1 ORDER BY position, created_at`,
		questionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	q.Options = []optionOut{}
	for rows.Next() {
		var o optionOut
		if err := rows.Scan(&o.ID, &o.Position, &o.Text, &o.IsCorrect); err != nil {
			return nil, err
		}
		if !includeCorrect {
			o.IsCorrect = false
		}
		q.Options = append(q.Options, o)
	}
	return &q, nil
}
