package services

import (
	"context"
	"log/slog"
	"time"

	"reserva/internal/core/ports"
)

type CleanupWorker struct {
	reservationRepo ports.ReservationRepository
	bookingService  ports.BookingService
	interval        time.Duration
}

func NewCleanupWorker(
	reservationRepo ports.ReservationRepository,
	bookingService ports.BookingService,
	interval time.Duration,
) *CleanupWorker {
	return &CleanupWorker{
		reservationRepo: reservationRepo,
		bookingService:  bookingService,
		interval:        interval,
	}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.processExpiredReservations(ctx)
			}
		}
	}()
}

func (w *CleanupWorker) processExpiredReservations(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in cleanup worker", "recover", r)
		}
	}()

	reservations, err := w.reservationRepo.GetExpiredReservations(ctx)
	if err != nil {
		slog.Error("Failed to fetch expired reservations", "error", err)
		return
	}

	for _, res := range reservations {
		slog.Info("Cancelling expired reservation", "reservation_id", res.ID)
		if err := w.bookingService.CancelReservation(ctx, res.ID, res.UserID); err != nil {
			slog.Error("Failed to cancel expired reservation", "reservation_id", res.ID, "error", err)
		}
	}
}
