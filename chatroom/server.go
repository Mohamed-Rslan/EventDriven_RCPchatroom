package main

import (
    "fmt"
    "log"
    "net"
    "net/rpc"
    "chatroom/commons"
)

func main() {
    chatRoom := NewChatRoom()

    err := rpc.Register(chatRoom)
    if err != nil {
        log.Fatal("Error registering chat room service:", err)
    }

    listener, err := net.Listen("tcp", commons.Get_server_address())
    if err != nil {
        log.Fatal("Error starting server:", err)
    }
    defer listener.Close()

    fmt.Println("Server is listening on", commons.Get_server_address())

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Fatal("Error accepting connection:", err)
        }

        go rpc.ServeConn(conn)
    }
}
