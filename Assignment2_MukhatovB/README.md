# Assignment 2 – Basic Web Server (Advanced Programming 2)

## Features

- POST /data: Store JSON key-value in-memory
- GET /data: Retrieve all stored data
- DELETE /data/{key}: Delete entry by key
- GET /stats: Show total requests handled
- Background logging every 5 seconds
- Graceful shutdown with signal handling

## Run the Server

```
go run main.go
```

Then use Postman or curl to test endpoints.

## Author
Mukhatov B.