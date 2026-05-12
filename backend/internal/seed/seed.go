package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// EnsureAdmin создаёт суперадмина при первом старте и поддерживает его
// пароль/роль в соответствии с .env: если значение поменяли — обновим.
func EnsureAdmin(ctx context.Context, pool *pgxpool.Pool, username, password, role string) error {
	var (
		id           uuid.UUID
		existingHash string
		existingRole string
	)
	err := pool.QueryRow(ctx,
		`SELECT id, password_hash, role FROM admins WHERE username=$1`,
		username,
	).Scan(&id, &existingHash, &existingRole)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash password: %w", hashErr)
		}
		_, err = pool.Exec(ctx,
			`INSERT INTO admins (username, password_hash, role) VALUES ($1, $2, $3)`,
			username, string(hash), role,
		)
		return err
	case err != nil:
		return fmt.Errorf("lookup admin: %w", err)
	}

	needPasswordUpdate := bcrypt.CompareHashAndPassword([]byte(existingHash), []byte(password)) != nil
	needRoleUpdate := existingRole != role
	if !needPasswordUpdate && !needRoleUpdate {
		return nil
	}

	newHash := existingHash
	if needPasswordUpdate {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash password: %w", hashErr)
		}
		newHash = string(hash)
	}
	_, err = pool.Exec(ctx,
		`UPDATE admins SET password_hash=$1, role=$2 WHERE id=$3`,
		newHash, role, id,
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

	var superID uuid.UUID
	_ = pool.QueryRow(ctx,
		`SELECT id FROM admins WHERE role='superadmin' ORDER BY created_at LIMIT 1`,
	).Scan(&superID)

	err = pool.QueryRow(ctx,
		`INSERT INTO variants (subject_id, title, duration_minutes, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		mathID, "Демо-вариант №1", 30, nullableUUID(superID),
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

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
