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
	if len(s.seatA) < seatLimitA {
		seat = fmt.Sprintf("A%d", len(s.seatA)+1)
		s.seatA = append(s.seatA, req.User.Email)
	} else if len(s.seatB) < seatLimitB {
		seat = fmt.Sprintf("B%d", len(s.seatB)+1)
		s.seatB = append(s.seatB, req.User.Email)
	} else {
		return nil, errors.New("no seats available")
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

// Mark a seat as vacant by setting it to an empty string
func (s *server) vacateSeat(seats []string, email string) {
	for i, e := range seats {
		if e == email {
			seats[i] = "" // Mark seat as vacant
			break
		}
	}
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
		s.seatA = s.removeSeat(s.seatA, req.Email) // Update seatA list
	} else if receipt.Seat[0] == 'B' {
		s.seatB = s.removeSeat(s.seatB, req.Email) // Update seatB list
	}

	// Finally, remove the user from the users map
	delete(s.users, req.Email)

	return &pb.Response{Message: "User removed successfully."}, nil
}

// ModifySeat modifies the seat of an existing user if the new seat is available
func (s *server) ModifySeat(ctx context.Context, req *pb.ModifyRequest) (*pb.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipt, exists := s.users[req.Email]
	if !exists {
		return nil, errors.New("user not found")
	}

	// Check if the new seat is in the correct section and is available
	if req.NewSeat[0] == 'A' {
		if len(s.seatA) >= seatLimitA || s.isSeatTaken(s.seatA, req.NewSeat) {
			return nil, errors.New("no vacant seat available in section A or seat is already taken")
		}
	} else if req.NewSeat[0] == 'B' {
		if len(s.seatB) >= seatLimitB || s.isSeatTaken(s.seatB, req.NewSeat) {
			return nil, errors.New("no vacant seat available in section B or seat is already taken")
		}
	} else {
		return nil, errors.New("invalid seat section")
	}

	// If new seat is available, proceed with removing current seat
	if receipt.Seat[0] == 'A' {
		s.seatA = s.removeSeat(s.seatA, req.Email)
	} else if receipt.Seat[0] == 'B' {
		s.seatB = s.removeSeat(s.seatB, req.Email)
	}

	// Reassign seat to the user
	receipt.Seat = req.NewSeat
	s.users[req.Email] = receipt

	// Add the user to the new seat allocation
	if req.NewSeat[0] == 'A' {
		s.seatA = append(s.seatA, req.Email)
	} else if req.NewSeat[0] == 'B' {
		s.seatB = append(s.seatB, req.Email)
	}

	return &pb.Response{Message: "Seat modified successfully."}, nil
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

// Utility function to fully remove a seat without leaving a vacant spot
func (s *server) removeSeat(seats []string, email string) []string {
	for i, e := range seats {
		if e == email {
			// Remove the user from the seat list and return the updated list
			return append(seats[:i], seats[i+1:]...)
		}
	}
	return seats
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
