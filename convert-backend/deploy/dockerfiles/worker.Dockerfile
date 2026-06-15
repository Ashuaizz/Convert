FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN go build -o /out/worker ./cmd/worker

FROM alpine:3.20
COPY --from=build /out/worker /usr/local/bin/worker
ENTRYPOINT ["worker"]
