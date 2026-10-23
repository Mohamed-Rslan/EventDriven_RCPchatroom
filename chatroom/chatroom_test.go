package main

import (
    "fmt"
    "net"
    "net/rpc"
    "sync"
    "testing"
    "chatroom/commons"
)

func setupTestServer(t *testing.T) (*rpc.Server, string, func()) {
    chatRoom := NewChatRoom()
    rpcServer := rpc.NewServer()
    err := rpcServer.Register(chatRoom)
    if err != nil {
        t.Fatalf("Failed to register ChatRoom: %v", err)
    }

    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("Failed to listen: %v", err)
    }

    done := make(chan struct{})
    go func() {
        for {
            conn, err := ln.Accept()
            if err != nil {
                select {
                case <-done:
                    return
                default:
                    return
                }
            }
            go rpcServer.ServeConn(conn)
        }
    }()

    cleanup := func() {
        close(done)
        _ = ln.Close()
    }

    return rpcServer, ln.Addr().String(), cleanup
}

func TestConcurrentRegistrationAndBroadcast(t *testing.T) {
    _, addr, cleanup := setupTestServer(t)
    defer cleanup()

    numClients := 30
    clients := make([]*rpc.Client, numClients)

    var wg sync.WaitGroup
    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            cli, err := rpc.Dial("tcp", addr)
            if err != nil {
                t.Errorf("Dial error for client %d: %v", id, err)
                return
            }
            nick := fmt.Sprintf("test_user_%d", id)
            var reply string
            err = cli.Call("ChatRoom.RegisterClient", &commons.RegisterArgs{Nickname: nick}, &reply)
            if err != nil {
                t.Errorf("Register error for client %d: %v", id, err)
                return
            }
            clients[id] = cli
        }(i)
    }
    wg.Wait()

    var bcastReply string
    err := clients[0].Call("ChatRoom.BroadcastMessage", &commons.MessageArgs{
        Nickname: "test_user_0",
        Message:  "Hello concurrency test!",
    }, &bcastReply)
    if err != nil {
        t.Fatalf("Broadcast failed: %v", err)
    }

    for _, cli := range clients {
        if cli != nil {
            _ = cli.Close()
        }
    }
}

func TestUnregisterClient(t *testing.T) {
    _, addr, cleanup := setupTestServer(t)
    defer cleanup()

    cli, err := rpc.Dial("tcp", addr)
    if err != nil {
        t.Fatalf("Dial failed: %v", err)
    }
    defer cli.Close()

    var regReply string
    err = cli.Call("ChatRoom.RegisterClient", &commons.RegisterArgs{Nickname: "leaver"}, &regReply)
    if err != nil {
        t.Fatalf("Register failed: %v", err)
    }

    var unregReply string
    err = cli.Call("ChatRoom.UnregisterClient", "leaver", &unregReply)
    if err != nil {
        t.Fatalf("Unregister failed: %v", err)
    }

    var listenReply string
    err = cli.Call("ChatRoom.ListenForMessages", "leaver", &listenReply)
    if err == nil {
        t.Fatalf("Expected error when listening after unregister, got nil")
    }
}
