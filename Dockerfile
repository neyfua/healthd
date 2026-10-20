# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /healthd ./cmd/healthd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /healthd /healthd
USER 65532:65532
EXPOSE 8080
ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/healthd", "-healthcheck"]
LABEL org.opencontainers.image.title="healthd" \
      org.opencontainers.image.description="Synthetic clinic API with an embedded dashboard" \
      org.opencontainers.image.source="https://github.com/neyfua/healthd" \
      org.opencontainers.image.licenses="MIT"
ENTRYPOINT ["/healthd"]
