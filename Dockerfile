FROM golang:1.18-buster as builder
WORKDIR /go/src/bolt
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -o /bolt cmd/main.go && chmod +x /bolt

FROM gcr.io/distroless/base
COPY --from=builder /bolt /bolt

CMD ["/bolt"]