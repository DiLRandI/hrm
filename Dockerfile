# Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json* ./frontend/
WORKDIR /app/frontend
RUN npm install
COPY frontend ./
RUN npm run build

# Build backend
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod .
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN go build -o /app/hrm ./cmd/server

# Final image
FROM alpine:3.19
WORKDIR /app
RUN adduser -D appuser
COPY --from=backend /app/hrm /app/hrm
COPY --from=backend /app/migrations /app/migrations
COPY --from=frontend /app/frontend/dist /app/frontend/dist
RUN mkdir -p /app/storage && chown -R appuser:appuser /app
USER appuser
ENV FRONTEND_DIR=/app/frontend/dist
EXPOSE 8080
CMD ["/app/hrm"]
