# Stage 1 — build
FROM golang:1.24-alpine AS build

RUN apk add --no-cache nodejs npm git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build frontend
RUN cd web && npm ci && npm run build

# Build Go binary
RUN CGO_ENABLED=0 go build -o /devctl ./cmd/devctl/

# Stage 2 — runtime
FROM docker:27-cli

RUN apk add --no-cache bash ca-certificates

COPY --from=build /devctl /usr/local/bin/devctl
COPY --from=build /src/templates /opt/devctl/templates

COPY docker-entrypoint.sh /opt/devctl/docker-entrypoint.sh
RUN chmod +x /opt/devctl/docker-entrypoint.sh

WORKDIR /opt/devctl

EXPOSE 19800

ENTRYPOINT ["/opt/devctl/docker-entrypoint.sh"]
CMD ["serve"]
