package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jjulito/reserva/internal/core/domain"
)

type PostgresSeatRepository struct {
	db *pgxpool.Pool
}

func NewPostgresSeatRepository(db *pgxpool.Pool) *PostgresSeatRepository {
	return &PostgresSeatRepository{db: db}
}

func (r *PostgresSeatRepository) GetSeat(ctx context.Context, seatID uuid.UUID) (*domain.Seat, error) {
	query := `SELECT id, event_id, seat_number, category, status, version, reserved_by FROM seats WHERE id = $1`
	row := r.db.QueryRow(ctx, query, seatID)

	var seat domain.Seat
	err := row.Scan(
		&seat.ID,
		&seat.EventID,
		&seat.Number,
		&seat.Category,
		&seat.Status,
		&seat.Version,
		&seat.ReservedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("seat not found")
		}
		return nil, err
	}
	return &seat, nil
}

func (r *PostgresSeatRepository) GetSeatsByEvent(ctx context.Context, eventID uuid.UUID) ([]domain.Seat, error) {
	query := `SELECT id, event_id, seat_number, category, status, version, reserved_by FROM seats WHERE event_id = $1`
	rows, err := r.db.Query(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []domain.Seat
	for rows.Next() {
		var seat domain.Seat
		err := rows.Scan(
			&seat.ID,
			&seat.EventID,
			&seat.Number,
			&seat.Category,
			&seat.Status,
			&seat.Version,
			&seat.ReservedBy,
		)
		if err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}
	return seats, nil
}

// UpdateSeatStatus updates the seat status designated by seatID, but only if the current version matches the provided version.
// This enforces optimistic locking to prevent race conditions.
func (r *PostgresSeatRepository) UpdateSeatStatus(ctx context.Context, seatID uuid.UUID, status domain.SeatStatus, reservedBy *uuid.UUID, currentVersion int) error {
	query := `
		UPDATE seats 
		SET status = $1, 
		    reserved_by = $2, 
		    version = version + 1 
		WHERE id = $3 AND version = $4
	`
	
	// Execute the update
	cmdTag, err := r.db.Exec(ctx, query, status, reservedBy, seatID, currentVersion)
	if err != nil {
		return err
	}

	// If no rows were affected, it means the version did not match (Optimistic Lock Failure)
	// or the seat with that ID doesn't exist.
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("optimistic lock failure: seat modified concurrently")
	}

	return nil
}
