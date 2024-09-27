FROM golang:1.23-bookworm as builder
WORKDIR /go/src/bolt
COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bolt cmd/main.go && chmod +x /bolt

FROM gcr.io/distroless/base
COPY --from=builder /bolt /bolt

CMD ["/bolt"]
