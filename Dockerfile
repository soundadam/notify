# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/notify ./cmd/notify

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/notify /notify
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/notify"]
