FROM golang:1.23-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# Без :nonroot — процесс root внутри контейнера, иначе named volume для загрузок
# часто root:root и os.Create в UPLOAD_DIR даёт permission denied.
FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
