# Ledger Application

A Go-based ledger application for tracking user balances and transactions.

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 14
- Make sure PostgreSQL service is running

## Environment Setup

1. Copy the `.env.example` to `.env` and update the values:

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=ledger_db
SERVER_PORT=8080
```

## Running the Application

1. Install dependencies:

```bash
go mod download
```

2. Run the application:

```bash
go run main.go
```

## API Endpoints

### Create User

- **POST** `/api/users`

```json
{
  "name": "John Doe"
}
```

More endpoints coming soon...

## Development

The project follows standard Go project layout:

```
.
├── README.md
├── go.mod
├── go.sum
├── main.go
└── internal/
    ├── api/
    │   └── server.go
    ├── models/
    │   ├── user.go
    │   └── transaction.go
    └── db/
        └── db.go
```
