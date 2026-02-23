FROM docker.io/golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/jotd ./cmd/jotd

FROM docker.io/alpine:3.20
RUN adduser -D -H app
USER app
COPY --from=build /out/jotd /usr/local/bin/jotd
EXPOSE 8080 9090
ENTRYPOINT ["/usr/local/bin/jotd"]
