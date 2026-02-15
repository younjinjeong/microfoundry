FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN CGO_ENABLED=0 go build -o /server main.go

FROM alpine:3.19
COPY --from=builder /server /server
ENV PORT=8080
EXPOSE 8080
CMD ["/server"]
