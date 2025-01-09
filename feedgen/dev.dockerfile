FROM golang:1.23-alpine
ENV GO111MODULE=on
WORKDIR /app
RUN apk add git
RUN go install github.com/air-verse/air@latest
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
CMD ["air", "-c", "air.toml"]