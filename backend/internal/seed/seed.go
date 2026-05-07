package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func EnsureAdmin(ctx context.Context, pool *pgxpool.Pool, username, password string) error {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM admins WHERE username=$1`, username).Scan(&id)
	if err == nil {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO admins (username, password_hash) VALUES ($1, $2)`,
		username, string(hash),
	)
	return err
}

// EnsureDemo создаёт минимальный демо-набор: предметы Математика и Информатика,
// у Математики — один вариант с двумя вопросами (один с одним правильным ответом,
// второй — с несколькими). Идемпотентно по имени предмета и заголовку варианта.
func EnsureDemo(ctx context.Context, pool *pgxpool.Pool) error {
	mathID, err := upsertSubject(ctx, pool, "Математика")
	if err != nil {
		return err
	}
	if _, err := upsertSubject(ctx, pool, "Информатика"); err != nil {
		return err
	}

	var variantID uuid.UUID
	err = pool.QueryRow(ctx,
		`SELECT id FROM variants WHERE subject_id=$1 AND title=$2`,
		mathID, "Демо-вариант №1",
	).Scan(&variantID)
	if err == nil {
		return nil
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO variants (subject_id, title, duration_minutes)
		 VALUES ($1, $2, $3) RETURNING id`,
		mathID, "Демо-вариант №1", 30,
	).Scan(&variantID)
	if err != nil {
		return fmt.Errorf("insert variant: %w", err)
	}

	type opt struct {
		text    string
		correct bool
	}
	type qd struct {
		text    string
		options []opt
	}
	demo := []qd{
		{
			text: "Сколько будет 2 + 2 × 2?",
			options: []opt{
				{"4", false},
				{"6", true},
				{"8", false},
				{"10", false},
			},
		},
		{
			text: "Какие из чисел являются простыми?",
			options: []opt{
				{"2", true},
				{"3", true},
				{"4", false},
				{"9", false},
				{"11", true},
			},
		},
		{
			text: "Решите уравнение: x² = 9. Какие значения подходят?",
			options: []opt{
				{"-3", true},
				{"0", false},
				{"3", true},
				{"9", false},
			},
		},
	}
	for i, q := range demo {
		var qid uuid.UUID
		err := pool.QueryRow(ctx,
			`INSERT INTO questions (variant_id, position, text)
			 VALUES ($1, $2, $3) RETURNING id`,
			variantID, i+1, q.text,
		).Scan(&qid)
		if err != nil {
			return fmt.Errorf("insert question: %w", err)
		}
		for j, o := range q.options {
			_, err := pool.Exec(ctx,
				`INSERT INTO answer_options (question_id, position, text, is_correct)
				 VALUES ($1, $2, $3, $4)`,
				qid, j+1, o.text, o.correct,
			)
			if err != nil {
				return fmt.Errorf("insert option: %w", err)
			}
		}
	}
	return nil
}

func upsertSubject(ctx context.Context, pool *pgxpool.Pool, name string) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subjects WHERE name=$1`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO subjects (name) VALUES ($1) RETURNING id`, name,
	).Scan(&id)
	return id, err
}
