FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN go build -o /out/pdf-service ./cmd/pdf-service

FROM alpine:3.20
COPY --from=build /out/pdf-service /usr/local/bin/pdf-service
EXPOSE 9001
ENTRYPOINT ["pdf-service"]
