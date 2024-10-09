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
