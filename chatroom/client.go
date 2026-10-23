package main

import (
    "bufio"
    "fmt"
    "log"
    "net/rpc"
    "os"
    "strings"
    "chatroom/commons"
)

func SendMessage(client *rpc.Client, nickname string) {
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        message := strings.TrimSpace(scanner.Text())

        if message == "" {
            continue
        }

        if message == "/quit" || message == "/exit" {
            var reply string
            _ = client.Call("ChatRoom.UnregisterClient", nickname, &reply)
            fmt.Println("Disconnected from chatroom.")
            os.Exit(0)
        }

        args := &commons.MessageArgs{
            Nickname: nickname,
            Message:  message,
        }
        var reply string
        err := client.Call("ChatRoom.BroadcastMessage", args, &reply)
        if err != nil {
            log.Println("Error sending message:", err)
            return
        }
    }
}

func ListenForMessages(client *rpc.Client, nickname string) {
    for {
        var reply string
        err := client.Call("ChatRoom.ListenForMessages", nickname, &reply)
        if err != nil {
            log.Println("Disconnected from server.")
            os.Exit(0)
        }
        fmt.Println(reply)
    }
}

func main() {
    client, err := rpc.Dial("tcp", commons.Get_server_address())
    if err != nil {
        log.Fatal("Error connecting to server:", err)
    }
    defer client.Close()

    fmt.Print("Enter your nickname: ")
    reader := bufio.NewReader(os.Stdin)
    rawNickname, err := reader.ReadString('\n')
    if err != nil {
        log.Fatal("Error reading nickname:", err)
    }
    nickname := strings.TrimSpace(rawNickname)
    if nickname == "" {
        log.Fatal("Nickname cannot be empty.")
    }

    registerArgs := &commons.RegisterArgs{Nickname: nickname}
    var registerReply string
    err = client.Call("ChatRoom.RegisterClient", registerArgs, &registerReply)
    if err != nil {
        log.Fatal("Error registering with server:", err)
    }
    fmt.Println(registerReply)
    fmt.Println("Type messages to broadcast (or '/quit' to exit):")

    go ListenForMessages(client, nickname)

    SendMessage(client, nickname)
}
