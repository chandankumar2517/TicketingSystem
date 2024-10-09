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
	seatLimitA = 10 // Seat capacity for section A
	seatLimitB = 10 // Seat capacity for section B
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
			receipt := s.users[email]
			users = append(users, &pb.UserSeatInfo{
				User: receipt.User,
				Seat: fmt.Sprintf("A%d", i+1),
			})
		}
	case "B":
		for i, email := range s.seatB {
			receipt := s.users[email]
			users = append(users, &pb.UserSeatInfo{
				User: receipt.User,
				Seat: fmt.Sprintf("B%d", i+1),
			})
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

	// Remove seat assignment
	if receipt.Seat[0] == 'A' {
		s.removeSeat(s.seatA, req.Email)
	} else if receipt.Seat[0] == 'B' {
		s.removeSeat(s.seatB, req.Email)
	}

	delete(s.users, req.Email)
	return &pb.Response{Message: "User removed successfully."}, nil
}

// ModifySeat modifies the seat of an existing user
func (s *server) ModifySeat(ctx context.Context, req *pb.ModifyRequest) (*pb.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipt, exists := s.users[req.Email]
	if !exists {
		return nil, errors.New("user not found")
	}

	// Remove current seat assignment
	if receipt.Seat[0] == 'A' {
		s.removeSeat(s.seatA, req.Email)
	} else if receipt.Seat[0] == 'B' {
		s.removeSeat(s.seatB, req.Email)
	}

	// Reassign seat
	receipt.Seat = req.NewSeat
	s.users[req.Email] = receipt

	return &pb.Response{Message: "Seat modified successfully."}, nil
}

// Utility function to remove a seat
func (s *server) removeSeat(seats []string, email string) {
	for i, e := range seats {
		if e == email {
			seats = append(seats[:i], seats[i+1:]...)
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
