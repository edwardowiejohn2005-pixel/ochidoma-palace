FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /ochidoma-server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /ochidoma-server ./ochidoma-server
COPY migrations ./migrations
EXPOSE 8080
ENTRYPOINT ["./ochidoma-server"]
