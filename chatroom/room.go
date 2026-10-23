package main

import (
    "fmt"
    "sync"
    "chatroom/commons"
)

const ClientChannelBuffer = 512

type ChatRoom struct {
    mu      sync.RWMutex
    clients map[string]chan string
}

func NewChatRoom() *ChatRoom {
    return &ChatRoom{
        clients: make(map[string]chan string),
    }
}

// safeSend delivers a message to a channel non-blockingly and recovers if the channel was closed concurrently.
func safeSend(ch chan string, msg string) (sent bool) {
    defer func() {
        if r := recover(); r != nil {
            sent = false
        }
    }()

    select {
    case ch <- msg:
        return true
    default:
        return false
    }
}

// RegisterClient registers a client with a buffered channel and notifies existing clients.
func (c *ChatRoom) RegisterClient(args *commons.RegisterArgs, reply *string) error {
    if args.Nickname == "" {
        return fmt.Errorf("nickname cannot be empty")
    }

    c.mu.Lock()
    // If client already registered, close old channel cleanly
    if oldCh, exists := c.clients[args.Nickname]; exists {
        delete(c.clients, args.Nickname)
        close(oldCh)
    }

    clientChan := make(chan string, ClientChannelBuffer)
    c.clients[args.Nickname] = clientChan

    // Snapshot other clients' channels while holding lock
    targets := make([]chan string, 0, len(c.clients))
    for nickname, ch := range c.clients {
        if nickname != args.Nickname {
            targets = append(targets, ch)
        }
    }
    c.mu.Unlock()

    *reply = fmt.Sprintf("%s registered successfully!", args.Nickname)

    // Notify other clients non-blockingly OUTSIDE the lock
    joinMsg := fmt.Sprintf("%s joined the chat!", args.Nickname)
    for _, ch := range targets {
        safeSend(ch, joinMsg)
    }

    return nil
}

// BroadcastMessage sends a message to all registered clients except the sender without holding locks.
func (c *ChatRoom) BroadcastMessage(args *commons.MessageArgs, reply *string) error {
    c.mu.RLock()
    // Snapshot recipients quickly under read-lock
    targets := make([]chan string, 0, len(c.clients))
    for nickname, ch := range c.clients {
        if nickname != args.Nickname {
            targets = append(targets, ch)
        }
    }
    c.mu.RUnlock()

    message := fmt.Sprintf("%s says: \"%s\"", args.Nickname, args.Message)

    // Non-blocking fan-out delivery
    for _, ch := range targets {
        safeSend(ch, message)
    }

    *reply = "Message broadcasted"
    return nil
}

// ListenForMessages retrieves the next message for the client from their dedicated channel.
func (c *ChatRoom) ListenForMessages(nickname string, reply *string) error {
    c.mu.RLock()
    clientChan, exists := c.clients[nickname]
    c.mu.RUnlock()

    if !exists {
        *reply = "Client not registered"
        return fmt.Errorf("client not registered")
    }

    msg, ok := <-clientChan
    if !ok {
        *reply = "Client disconnected"
        return fmt.Errorf("client channel closed")
    }

    *reply = msg
    return nil
}

// UnregisterClient cleans up a client's registration and closes their channel.
func (c *ChatRoom) UnregisterClient(nickname string, reply *string) error {
    c.mu.Lock()
    ch, exists := c.clients[nickname]
    if exists {
        delete(c.clients, nickname)
        close(ch)
    }

    targets := make([]chan string, 0, len(c.clients))
    for _, clientCh := range c.clients {
        targets = append(targets, clientCh)
    }
    c.mu.Unlock()

    if exists {
        leaveMsg := fmt.Sprintf("%s left the chat.", nickname)
        for _, clientCh := range targets {
            safeSend(clientCh, leaveMsg)
        }
    }

    *reply = fmt.Sprintf("%s unregistered successfully", nickname)
    return nil
}
