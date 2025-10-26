FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG GQLGEN_VERSION=v0.17.81
RUN go install github.com/99designs/gqlgen@${GQLGEN_VERSION} && \
    gqlgen generate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /bin/myApi ./cmd/myApi


FROM gcr.io/distroless/base-debian12
COPY --from=builder /bin/myApi /myApi
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/myApi"]