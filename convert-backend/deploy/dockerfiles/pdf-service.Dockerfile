FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -o /out/pdf-service ./cmd/pdf-service

FROM alpine:3.20
COPY --from=build /out/pdf-service /usr/local/bin/pdf-service
EXPOSE 9001
ENTRYPOINT ["pdf-service"]
