package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jjulito/reserva/internal/core/domain"
)

type PostgresEventRepository struct {
	db *pgxpool.Pool
}

func NewPostgresEventRepository(db *pgxpool.Pool) *PostgresEventRepository {
	return &PostgresEventRepository{db: db}
}

func (r *PostgresEventRepository) GetEvent(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	query := `SELECT id, name, total_seats, is_active FROM events WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)

	var event domain.Event
	err := row.Scan(&event.ID, &event.Name, &event.TotalSeats, &event.IsActive)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, err
	}
	return &event, nil
}

func (r *PostgresEventRepository) ListEvents(ctx context.Context) ([]domain.Event, error) {
	query := `SELECT id, name, total_seats, is_active FROM events WHERE is_active = true`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(&event.ID, &event.Name, &event.TotalSeats, &event.IsActive); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
