package main



/*

TODO
the server has either two implementations
1) pooling
	- every message sent to the server has to be stored in long list
	- a client may ask for this list or a slice of it to fetch the updates


2) event-driven [BONUS]
	- a server is more like a coordinator
	- the server waits for clients wanting to register themselves as listeners
	- a client sends a message by calling an rpc responsible for broadcasting, the client calls a function to loop on all registered clients and send his own message to each of them separately
	- the client on the other side has a server listening for messages being pushed
*/
// server.go

// server.go
// server.go


import (
    "fmt"
    "log"
    "net"
    "net/rpc"
    "sync"
    "chatroom/commons"
)

type ChatRoom struct {
    mu      sync.Mutex
    clients map[string]chan string
}

func (c *ChatRoom) RegisterClient(args *commons.RegisterArgs, reply *string) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.clients[args.Nickname] = make(chan string)
    *reply = fmt.Sprintf("%s registered successfully!", args.Nickname)

    for nickname, clientChan := range c.clients {
        if nickname != args.Nickname {
            clientChan <- fmt.Sprintf("%s joined the chat!", args.Nickname)
        }
    }

    return nil
}

func (c *ChatRoom) BroadcastMessage(args *commons.MessageArgs, reply *string) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    message := fmt.Sprintf("%s says: \"%s\"", args.Nickname, args.Message)
    for nickname, clientChan := range c.clients {
        if nickname != args.Nickname {
            clientChan <- message
        }
    }

    *reply = "Message broadcasted"
    return nil
}

func (c *ChatRoom) ListenForMessages(nickname string, reply *string) error {
    c.mu.Lock()
    clientChan, exists := c.clients[nickname]
    c.mu.Unlock()

    if !exists {
        *reply = "Client not registered"
        return fmt.Errorf("client not registered")
    }

    *reply = <-clientChan
    return nil
}

func NewChatRoom() *ChatRoom {
    return &ChatRoom{
        clients: make(map[string]chan string),
    }
}

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
