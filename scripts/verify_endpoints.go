package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const baseURL = "http://localhost:8080/api/v1"

func main() {
	fmt.Println("Starting verification...")

	// 1. List Events
	events := listEvents()
	if len(events) == 0 {
		fatal("No events found")
	}
	eventID := events[0]["id"].(string)
	fmt.Printf("[OK] Found event: %s\n", eventID)

	// 2. Get Seats
	seats := getSeats(eventID)
	var seatID string
	for _, s := range seats {
		if s["status"].(string) == "AVAILABLE" {
			seatID = s["id"].(string)
			break
		}
	}
	if seatID == "" {
		fatal("No available seats found")
	}
	fmt.Printf("[OK] Found available seat: %s\n", seatID)

	// 3. Create Reservation
	userID := "550e8400-e29b-41d4-a716-446655440000" // Mock User ID
	reservation := createReservation(userID, seatID, eventID)
	resID := reservation["id"].(string)
	fmt.Printf("[OK] Created reservation: %s\n", resID)

	// 4. Get Reservation
	resCheck := getReservation(resID)
	if resCheck["status"] != "PENDING" {
		fatal(fmt.Sprintf("Expected status PENDING, got %s", resCheck["status"]))
	}
	fmt.Println("[OK] Verified reservation status is PENDING")

	// 5. Cancel Reservation
	cancelReservation(resID)
	fmt.Println("[OK] Cancelled reservation")

	// 6. Verify Cancellation
	resCancelled := getReservation(resID)
	if resCancelled["status"] != "CANCELLED" {
		fatal(fmt.Sprintf("Expected status CANCELLED, got %s", resCancelled["status"]))
	}
	fmt.Println("[OK] Verified reservation status is CANCELLED")

	fmt.Println("SUCCESS! All checks passed.")
}

func listEvents() []map[string]interface{} {
	resp, err := http.Get(baseURL + "/events")
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	var events []map[string]interface{}
	decode(resp.Body, &events)
	return events
}

func getSeats(eventID string) []map[string]interface{} {
	resp, err := http.Get(baseURL + "/events/" + eventID + "/seats")
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	var seats []map[string]interface{}
	decode(resp.Body, &seats)
	return seats
}

func createReservation(userID, seatID, eventID string) map[string]interface{} {
	body := map[string]string{
		"user_id":  userID,
		"seat_id":  seatID,
		"event_id": eventID,
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(baseURL+"/reservations", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("Create failed: %s", string(b)))
	}
	var res map[string]interface{}
	decode(resp.Body, &res)
	return res
}

func getReservation(id string) map[string]interface{} {
	resp, err := http.Get(baseURL + "/reservations/" + id)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fatal("GetReservation failed")
	}
	var res map[string]interface{}
	decode(resp.Body, &res)
	return res
}

func cancelReservation(id string) {
	resp, err := http.Post(baseURL+"/reservations/"+id+"/cancel", "application/json", nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("Cancel failed: %s", string(b)))
	}
}

func decode(r io.Reader, v interface{}) {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Println("ERROR:", msg)
	os.Exit(1)
}
