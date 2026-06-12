FROM node:22-alpine AS admin-builder

WORKDIR /src/apps/admin

RUN npm install -g pnpm@11

COPY apps/admin/package.json apps/admin/pnpm-lock.yaml apps/admin/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY apps/admin ./
RUN pnpm build

FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -o /out/svc ./cmd/main.go

FROM alpine:latest

RUN apk add --no-cache --upgrade ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /out/svc ./svc
COPY --from=admin-builder /src/apps/admin/dist ./admin-dist
COPY ./docs ./docs
COPY ./migrations ./migrations

CMD ["./svc"]
