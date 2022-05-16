# syntax=docker/dockerfile:1
FROM golang:1.18

RUN mkdir /app
ADD ./billing /app/billing
RUN mkdir -p /app/event_schema_registry/schemas
COPY ../schemas /app/event_schema_registry/schemas
WORKDIR /app/billing
RUN cd /app/billing; go mod download && go mod verify

EXPOSE 3003
