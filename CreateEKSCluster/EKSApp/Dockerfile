# Start from the latest golang base image
FROM golang:1.21.11

# Add Maintainer Info
LABEL maintainer="Jared"

# Set the Current Working Directory inside the container
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o main .


# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./main"] 