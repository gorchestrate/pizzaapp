FROM alpine:latest
RUN apk add ca-certificates
COPY pizzaapp /pizzaapp
CMD ["/pizzaapp"]
