package handlers

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// FinishAbandoned завершает все попытки, начатые более 24 часов назад.
// Вызывается при старте сервера и периодически в фоне.
func (h *PublicHandler) FinishAbandoned(ctx context.Context) {
	rows, err := h.Pool.Query(ctx, `
		SELECT id, variant_id FROM attempts
		WHERE finished_at IS NULL AND started_at < now() - interval '24 hours'
	`)
	if err != nil {
		log.Printf("finishAbandoned: query: %v", err)
		return
	}

	type entry struct{ id, variantID uuid.UUID }
	var batch []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.variantID); err != nil {
			log.Printf("finishAbandoned: scan: %v", err)
			rows.Close()
			return
		}
		batch = append(batch, e)
	}
	rows.Close()

	now := time.Now()
	for _, e := range batch {
		score, total, _, err := computeAttemptReview(ctx, h.Pool, e.id, e.variantID)
		if err != nil {
			log.Printf("finishAbandoned: score attempt %s: %v", e.id, err)
			continue
		}
		if _, err := h.Pool.Exec(ctx,
			`UPDATE attempts SET finished_at=$1, score=$2, total=$3 WHERE id=$4 AND finished_at IS NULL`,
			now, score, total, e.id,
		); err != nil {
			log.Printf("finishAbandoned: update attempt %s: %v", e.id, err)
		}
	}
	if len(batch) > 0 {
		log.Printf("finishAbandoned: closed %d abandoned attempts", len(batch))
	}
}

// maybeAutoFinish проверяет одну попытку: если она висит 24ч+ — завершает её.
// Используется как ленивая проверка при GET /attempts/{id}.
func (h *PublicHandler) maybeAutoFinish(ctx context.Context, id uuid.UUID) {
	var started time.Time
	var finished *time.Time
	var variantID uuid.UUID
	err := h.Pool.QueryRow(ctx,
		`SELECT started_at, finished_at, variant_id FROM attempts WHERE id=$1`, id,
	).Scan(&started, &finished, &variantID)
	if err != nil || finished != nil {
		return
	}
	if time.Since(started) < 24*time.Hour {
		return
	}
	score, total, _, err := computeAttemptReview(ctx, h.Pool, id, variantID)
	if err != nil {
		log.Printf("maybeAutoFinish %s: %v", id, err)
		return
	}
	if _, err := h.Pool.Exec(ctx,
		`UPDATE attempts SET finished_at=now(), score=$1, total=$2 WHERE id=$3 AND finished_at IS NULL`,
		score, total, id,
	); err != nil {
		log.Printf("maybeAutoFinish update %s: %v", id, err)
	}
}
