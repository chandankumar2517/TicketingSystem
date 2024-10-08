# TicketingSystem


A simple in-memory train ticketing system built with Go and gRPC, allowing users to purchase train tickets, view receipts, manage user allocations, and modify seat assignments.

## Project Overview

The **TicketingSystem** connects users from Source to Destination via train. This application enables users to purchase tickets and manage their bookings without the need for a persistent database, utilizing in-memory data structures instead. 

### Features

- Submit a ticket purchase with user details and seat allocation.
- Retrieve receipt details for purchased tickets.
- View users and their allocated seats by section.
- Remove a user from the train system.
- Modify a user's seat assignment.

## Technologies Used

- Go
- gRPC
- Protocol Buffers

## Installation

### Prerequisites

- Go (version 1.16 or later)
- Protocol Buffers (protoc)

### Setup

1. **Clone the Repository**:
   ```bash
   git clone [https://github.com/chandankumar2517/TicketingSystem.git](https://github.com/chandankumar2517/TicketingSystem.git)
   cd TicketingSystem
