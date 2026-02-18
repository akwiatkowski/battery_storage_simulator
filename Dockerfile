# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY simulator/frontend/package.json simulator/frontend/package-lock.json ./
RUN npm ci
COPY simulator/frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app/backend
COPY simulator/backend/go.mod simulator/backend/go.sum ./
RUN go mod download
COPY simulator/backend/ ./
RUN CGO_ENABLED=0 go build -o /server ./cmd/server/main.go

# Stage 3: Final image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=backend-builder /server ./server
COPY --from=frontend-builder /app/frontend/build ./frontend/build
EXPOSE 8080
CMD ["./server"]
