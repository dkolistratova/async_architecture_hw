# syntax=docker/dockerfile:1
FROM golang:1.18

RUN mkdir /app
ADD ./tasktracker /app/tasktracker
RUN mkdir -p /app/event_schema_registry/schemas
COPY ../schemas /app/event_schema_registry/schemas
WORKDIR /app/tasktracker
RUN cd /app/tasktracker; go mod download && go mod verify

EXPOSE 3002
