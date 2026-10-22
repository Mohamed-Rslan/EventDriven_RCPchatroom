package main


/*
TODO
there are two implementations
1) pooling
	- the client will dial the rpc of the coordinating server
	- the client will call the remote procedure on the server to send a message
	- the client can fetch all of the messages history from the server using remote procedure call


2) event-driven [BONUS]
	- a client starts by looking for a port to establish it's server on (like giving my phone number to my friends to call me)
	- a client can send a message through an infinite loop waiting for input text, this message will be broadcasted to other clients through an rpc call on the server
	- a client can also receive messages simultaneously using the GO keyword
	- so a client here is a server + a client at the same time
*/
// client.go

// client.go

// client.go


import (
    "fmt"
    "log"
    "net/rpc"
    "os"
    "chatroom/commons"
    "bufio"
)

func SendMessage(client *rpc.Client, nickname string) {
    for {
        scanner := bufio.NewScanner(os.Stdin)
        scanner.Scan()
        message := scanner.Text()

        if message != "" {
            args := &commons.MessageArgs{
                Nickname: nickname,
                Message:  message,
            }
            var reply string
            err := client.Call("ChatRoom.BroadcastMessage", args, &reply)
            if err != nil {
                log.Fatal("Error sending message:", err)
            }
        }
    }
}

func ListenForMessages(client *rpc.Client, nickname string) {
    for {
        var reply string
        err := client.Call("ChatRoom.ListenForMessages", nickname, &reply)
        if err != nil {
            log.Fatal("Error receiving message:", err)
        }
        fmt.Println(reply)
    }
}

func main() {
    client, err := rpc.Dial("tcp", commons.Get_server_address())
    if err != nil {
        log.Fatal("Error connecting to server:", err)
    }

    fmt.Print("Enter your nickname: ")
    var nickname string
    fmt.Scanln(&nickname)

    registerArgs := &commons.RegisterArgs{Nickname: nickname}
    var registerReply string
    err = client.Call("ChatRoom.RegisterClient", registerArgs, &registerReply)
    if err != nil {
        log.Fatal("Error registering with server:", err)
    }
    fmt.Println(registerReply)

    go ListenForMessages(client, nickname)

    SendMessage(client, nickname)
}


