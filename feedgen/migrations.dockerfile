FROM golang:1.23-alpine

# Install migrate tool
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

WORKDIR /migrations

# Copy migration files
COPY pkg/db/migrations/*.sql ./

# Run migrations
CMD ["sh", "-c", "migrate -path=. -database postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:5432/${POSTGRES_DB}?sslmode=disable up"]