FROM golang:1.25-alpine AS builder

ARG VERSION=dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.version=${VERSION}" -o /mf ./cmd/mf

# Terraform binary for CSP service provisioning
FROM hashicorp/terraform:1.9 AS terraform

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git curl \
    && curl -sLO "https://dl.k8s.io/release/$(curl -sL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && install kubectl /usr/local/bin/kubectl \
    && rm kubectl

COPY --from=terraform /bin/terraform /usr/local/bin/terraform
COPY --from=builder /mf /usr/local/bin/mf

EXPOSE 8080
ENTRYPOINT ["mf"]
