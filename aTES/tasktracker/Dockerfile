# syntax=docker/dockerfile:1
FROM golang:1.18

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

RUN mkdir /app
WORKDIR /app
ADD . /app
RUN go mod download && go mod verify

EXPOSE 3002
