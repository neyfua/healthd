# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /healthd ./cmd/healthd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /healthd /healthd
USER 65532:65532
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/healthd"]
