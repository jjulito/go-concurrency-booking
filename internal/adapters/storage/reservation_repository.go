package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jjulito/reserva/internal/core/domain"
)

type PostgresReservationRepository struct {
	db *pgxpool.Pool
}

func NewPostgresReservationRepository(db *pgxpool.Pool) *PostgresReservationRepository {
	return &PostgresReservationRepository{db: db}
}

func (r *PostgresReservationRepository) CreateReservation(ctx context.Context, res *domain.Reservation) error {
	query := `
		INSERT INTO reservations (id, user_id, seat_id, event_id, status, amount, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	// Assuming fixed amount for now or added to model
	amount := 10.00 

	_, err := r.db.Exec(ctx, query,
		res.ID,
		res.UserID,
		res.SeatID,
		res.EventID,
		res.Status,
		amount,
		res.CreatedAt,
		res.ExpiresAt,
	)
	return err
}

func (r *PostgresReservationRepository) GetReservation(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	query := `SELECT id, user_id, seat_id, event_id, status, created_at, expires_at FROM reservations WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)

	var res domain.Reservation
	err := row.Scan(
		&res.ID,
		&res.UserID,
		&res.SeatID,
		&res.EventID,
		&res.Status,
		&res.CreatedAt,
		&res.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("reservation not found")
		}
		return nil, err
	}
	return &res, nil
}

func (r *PostgresReservationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReservationStatus) error {
	query := `UPDATE reservations SET status = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, status, id)
	return err
}

func (r *PostgresReservationRepository) GetExpiredReservations(ctx context.Context) ([]domain.Reservation, error) {
	// Query for PENDING reservations where expires_at < NOW()
	query := `
		SELECT id, user_id, seat_id, event_id, status, created_at, expires_at 
		FROM reservations 
		WHERE status = 'PENDING' AND expires_at < NOW()
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reservations []domain.Reservation
	for rows.Next() {
		var res domain.Reservation
		err := rows.Scan(
			&res.ID,
			&res.UserID,
			&res.SeatID,
			&res.EventID,
			&res.Status,
			&res.CreatedAt,
			&res.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		reservations = append(reservations, res)
	}
	return reservations, nil
}
