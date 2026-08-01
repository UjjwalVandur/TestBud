FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /mock-server ./examples/mock-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=build /mock-server /usr/local/bin/mock-server
EXPOSE 8090
CMD ["mock-server"]
