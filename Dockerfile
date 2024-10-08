FROM golang:1.23-bookworm as builder
WORKDIR /go/src/bolt
COPY . .

# The "json1" build tag enables JSON SQL functions in go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags json1 -o /bolt cmd/main.go && chmod +x /bolt

FROM gcr.io/distroless/base
COPY --from=builder /bolt /bolt

CMD ["/bolt"]
