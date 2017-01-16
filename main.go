package main

import (
    "log"
    "bufio"
    "net/http"
    "github.com/googollee/go-socket.io"
)

func main() {
    server, err := socketio.NewServer(nil)
    if err != nil {
        log.Fatal(err)
    }

    server.On("connection", func(so socketio.Socket) {
        log.Println("on connection")
    })

    http.Handle("/socket.io/", server)
    http.Handle("/", http.FileServer(http.Dir("./asset")))
    log.Println("Serving at localhost:8080...")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
