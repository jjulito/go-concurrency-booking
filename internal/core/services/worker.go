package services

import (
	"context"
	"log"
	"time"

	"github.com/jjulito/reserva/internal/core/ports"
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
	reservations, err := w.reservationRepo.GetExpiredReservations(ctx)
	if err != nil {
		log.Printf("Error fetching expired reservations: %v", err)
		return
	}

	for _, res := range reservations {
		// Use the service to cancel, ensuring logic (releasing seats) is consistent
		log.Printf("Cancelling expired reservation: %s", res.ID)
		if err := w.bookingService.CancelReservation(ctx, res.ID); err != nil {
			log.Printf("Error cancelling reservation %s: %v", res.ID, err)
		}
	}
}
