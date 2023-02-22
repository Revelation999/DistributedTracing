FROM golang:latest
WORKDIR /app
ADD . /app
RUN go mod download
RUN go build -o hwk .
EXPOSE 8082
CMD ["/app/hwk"]