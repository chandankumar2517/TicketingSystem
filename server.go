package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/chandankumar2517/TrainTicketingSystem/train_ticketing/train_ticketing" // Import the generated package

	"google.golang.org/grpc"
)

const (
	seatLimitA = 5 // Seat capacity for section A
	seatLimitB = 5 // Seat capacity for section B
)

type server struct {
	pb.UnimplementedTicketServiceServer
	mu    sync.Mutex
	users map[string]pb.Receipt // Map of users by email to Receipt
	seatA []string              // Allocated seats in section A
	seatB []string              // Allocated seats in section B
}

// NewServer creates a new gRPC server instance
func NewServer() *server {
	return &server{
		users: make(map[string]pb.Receipt),
		seatA: []string{},
		seatB: []string{},
	}
}

// PurchaseTicket allocates a seat and returns a receipt
func (s *server) PurchaseTicket(ctx context.Context, req *pb.PurchaseRequest) (*pb.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[req.User.Email]; exists {
		return nil, errors.New("user already purchased a ticket")
	}

	// Allocate seat
	var seat string
	var allocated bool // To track if a seat has been allocated

	// Check for vacant seats in section A
	for i, email := range s.seatA {
		if email == "" { // Vacant seat found
			seat = fmt.Sprintf("A%d", i+1)
			s.seatA[i] = req.User.Email
			allocated = true
			break
		}
	}

	// If no vacant seats in section A, check for vacant seats in section B
	if !allocated {
		for i, email := range s.seatB {
			if email == "" { // Vacant seat found
				seat = fmt.Sprintf("B%d", i+1)
				s.seatB[i] = req.User.Email
				allocated = true
				break
			}
		}
	}

	// If no vacant seats, append to the end if space is available
	if !allocated {
		if len(s.seatA) < seatLimitA {
			seat = fmt.Sprintf("A%d", len(s.seatA)+1)
			s.seatA = append(s.seatA, req.User.Email)
		} else if len(s.seatB) < seatLimitB {
			seat = fmt.Sprintf("B%d", len(s.seatB)+1)
			s.seatB = append(s.seatB, req.User.Email)
		} else {
			return nil, errors.New("no seats available")
		}
	}

	receipt := &pb.Receipt{
		From:      req.From,
		To:        req.To,
		User:      req.User,
		PricePaid: req.PricePaid,
		Seat:      seat,
	}
	s.users[req.User.Email] = *receipt

	return receipt, nil
}

// Helper function to find a vacant seat in a section
func (s *server) findVacantSeat(seats []string, section string) string {
	for i, seat := range seats {
		if seat == "" { // If seat is vacant, return the seat number
			return fmt.Sprintf("%s%d", section, i+1)
		}
	}
	return ""
}

// Helper function to get the index of a seat
func (s *server) getSeatIndex(seat string) int {
	var index int
	fmt.Sscanf(seat[1:], "%d", &index) // Extract the seat number after the section letter
	return index - 1
}

// GetReceipt returns the receipt for a user by email
func (s *server) GetReceipt(ctx context.Context, req *pb.ReceiptRequest) (*pb.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipt, exists := s.users[req.Email]
	if !exists {
		return nil, errors.New("receipt not found for user")
	}

	return &receipt, nil
}

// GetAllocatedUsers returns users and their seats for a requested section
func (s *server) GetAllocatedUsers(ctx context.Context, req *pb.SectionRequest) (*pb.UserList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var users []*pb.UserSeatInfo
	switch req.Section {
	case "A":
		for i, email := range s.seatA {
			if email != "" { // Only add allocated seats
				receipt := s.users[email]
				users = append(users, &pb.UserSeatInfo{
					User: receipt.User,
					Seat: fmt.Sprintf("A%d", i+1),
				})
			}
		}
	case "B":
		for i, email := range s.seatB {
			if email != "" { // Only add allocated seats
				receipt := s.users[email]
				users = append(users, &pb.UserSeatInfo{
					User: receipt.User,
					Seat: fmt.Sprintf("B%d", i+1),
				})
			}
		}
	default:
		return nil, errors.New("invalid section")
	}

	return &pb.UserList{UserSeats: users}, nil
}

// RemoveUser removes a user from the train system
func (s *server) RemoveUser(ctx context.Context, req *pb.RemoveRequest) (*pb.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipt, exists := s.users[req.Email]
	if !exists {
		return nil, errors.New("user not found")
	}

	// Remove seat assignment based on section (A or B)
	if receipt.Seat[0] == 'A' {
		s.vacateSeat(s.seatA, req.Email) // Update seatA list
	} else if receipt.Seat[0] == 'B' {
		s.vacateSeat(s.seatB, req.Email) // Update seatB list
	}

	// Finally, remove the user from the users map
	delete(s.users, req.Email)

	return &pb.Response{Message: "User removed successfully."}, nil
}

// ModifySeat modifies the seat of an existing user if the new seat is available
func (s *server) ModifySeat(ctx context.Context, req *pb.ModifyRequest) (*pb.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the user exists
	receipt, exists := s.users[req.Email]
	if !exists {
		return nil, errors.New("user not found")
	}

	// If the user is requesting the same seat they are currently seated in, no modification is needed
	if receipt.Seat == req.NewSeat {
		return nil, errors.New("user is already seated in the requested seat")
	}

	var sectionSeats []string
	var seatLimit int

	// Determine the seat section (A or B) based on the requested seat
	if req.NewSeat[0] == 'A' {
		sectionSeats = s.seatA
		seatLimit = seatLimitA
	} else if req.NewSeat[0] == 'B' {
		sectionSeats = s.seatB
		seatLimit = seatLimitB
	} else {
		return nil, errors.New("invalid seat section")
	}

	// Check if the new seat is within valid seat range and not taken
	seatIndex := -1
	for i := 0; i < seatLimit; i++ {
		currentSeat := fmt.Sprintf("%c%d", req.NewSeat[0], i+1)
		if currentSeat == req.NewSeat {
			seatIndex = i
			if sectionSeats[i] != "" && sectionSeats[i] != req.Email {
				return nil, errors.New("the requested seat is already taken")
			}
			break
		}
	}

	// If seatIndex is -1, it means the requested seat is invalid or beyond seat limit
	if seatIndex == -1 {
		return nil, errors.New("invalid seat number")
	}

	// Vacate the current seat
	if receipt.Seat[0] == 'A' {
		s.vacateSeat(s.seatA, req.Email)
	} else if receipt.Seat[0] == 'B' {
		s.vacateSeat(s.seatB, req.Email)
	}

	// Assign the user to the new seat
	sectionSeats[seatIndex] = req.Email

	// Update the user's receipt with the new seat
	receipt.Seat = req.NewSeat
	s.users[req.Email] = receipt

	return &pb.Response{Message: "Seat modified successfully."}, nil
}

// Utility function to vacate the current seat
func (s *server) vacateSeat(seats []string, email string) {
	for i, e := range seats {
		if e == email {
			seats[i] = "" // Mark the seat as vacant
			break
		}
	}
}

// Utility function to vacate the current seat and move the user to the new seat in the same section
func (s *server) vacateSeatAndMoveToNewSeat(seats []string, email string, newSeatIndex int, newSeat string) []string {
	// Vacate the user's current seat
	for i, e := range seats {
		if e == email {
			seats[i] = "" // Mark seat as vacant
			break
		}
	}

	// Assign the user to the new seat
	if newSeatIndex != -1 {
		seats[newSeatIndex] = email // Reuse a vacant seat if available
	} else {
		// If no specific seat is mentioned, append the user to the seat list
		seats = append(seats, email)
	}

	return seats
}

// Utility function to check if a seat is already taken
func (s *server) isSeatTaken(seats []string, email string) bool {
	for _, e := range seats {
		if e == email {
			return true
		}
	}
	return false
}

// Utility function to mark a seat as vacant without rearranging
func (s *server) removeSeat(seats []string, email string) {
	for i, e := range seats {
		if e == email {
			seats[i] = "" // Mark seat as vacant instead of removing it
			break
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterTicketServiceServer(grpcServer, NewServer())

	log.Println("Server is running at :50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
