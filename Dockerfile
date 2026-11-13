# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

# Dependency cache kept separate from the source code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ride ./cmd/api

FROM alpine:3.23

RUN adduser -D -u 10001 ride
USER ride

COPY --from=build /out/ride /usr/local/bin/ride

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/ride"]
