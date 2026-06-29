FROM golang:1.26 AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o plexus-controller ./cmd/controller/

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/plexus-controller /plexus-controller
USER 65532:65532
ENTRYPOINT ["/plexus-controller"]
