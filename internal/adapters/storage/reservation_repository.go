package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reserva/internal/core/domain"
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
	_, err := getQuerier(ctx, r.db).Exec(ctx, query,
		res.ID,
		res.UserID,
		res.SeatID,
		res.EventID,
		res.Status,
		res.Amount,
		res.CreatedAt,
		res.ExpiresAt,
	)
	return err
}

func (r *PostgresReservationRepository) GetReservation(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	query := `SELECT id, user_id, seat_id, event_id, status, amount, created_at, expires_at FROM reservations WHERE id = $1`
	row := getQuerier(ctx, r.db).QueryRow(ctx, query, id)

	var res domain.Reservation
	err := row.Scan(
		&res.ID,
		&res.UserID,
		&res.SeatID,
		&res.EventID,
		&res.Status,
		&res.Amount,
		&res.CreatedAt,
		&res.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrReservationNotFound
		}
		return nil, err
	}
	return &res, nil
}

func (r *PostgresReservationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReservationStatus) error {
	query := `UPDATE reservations SET status = $1 WHERE id = $2`
	cmdTag, err := getQuerier(ctx, r.db).Exec(ctx, query, status, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrReservationNotFound
	}
	return nil
}

func (r *PostgresReservationRepository) GetExpiredReservations(ctx context.Context) ([]domain.Reservation, error) {
	query := `
		SELECT id, user_id, seat_id, event_id, status, amount, created_at, expires_at
		FROM reservations
		WHERE status = 'PENDING' AND expires_at < NOW()
	`
	rows, err := getQuerier(ctx, r.db).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reservations := []domain.Reservation{}
	for rows.Next() {
		var res domain.Reservation
		err := rows.Scan(
			&res.ID,
			&res.UserID,
			&res.SeatID,
			&res.EventID,
			&res.Status,
			&res.Amount,
			&res.CreatedAt,
			&res.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		reservations = append(reservations, res)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reservations, nil
}
