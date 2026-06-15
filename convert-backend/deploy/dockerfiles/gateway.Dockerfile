FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN go build -o /out/gateway ./cmd/gateway

FROM alpine:3.20
COPY --from=build /out/gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["gateway"]
