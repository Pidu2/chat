# Websocket Chat App
Playing around with RabbitMQ, Sqlite, Golang, Websockets. Obviously completely useless, badly designed and highly insecure.
## Run
* `podman run -d --name rabbitmq -p 5672:5672 -p 15672:15672 -e RABBITMQ_DEFAULT_USER=user -e RABBITMQ_DEFAULT_PASS=password rabbitmq:3-management`
* `go run` each of the components in cmd
```
