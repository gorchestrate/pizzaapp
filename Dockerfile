FROM golang:1.15 as build
WORKDIR /wd
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -tags netgo -ldflags "-w -extldflags \"-static\"" -o mail-plugin .


FROM alpine:latest
RUN apk add ca-certificates
COPY --from=build /wd/mail-plugin /mail-plugin
CMD ["/mail-plugin"]
