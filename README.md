# ⚡ Event-Driven Distributed RPC Chat Room

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/)
[![Protocol](https://img.shields.io/badge/Protocol-net%2Frpc%20TCP-blue?style=for-the-badge)](https://pkg.go.dev/net/rpc)
[![Architecture](https://img.shields.io/badge/Architecture-Event--Driven%20%2F%20Pub--Sub-success?style=for-the-badge)]()
[![Concurrency](https://img.shields.io/badge/Concurrency-Goroutines%20%26%20Channels-orange?style=for-the-badge)]()
[![Throughput](https://img.shields.io/badge/Throughput-47%2C700%2B%20msgs%2Fsec-brightgreen?style=for-the-badge)]()

A high-performance, deadlock-free, event-driven terminal chatroom implemented in **Go** using the native 
et/rpc framework over TCP. Features asynchronous message distribution, non-blocking fan-out broadcasting, and comprehensive concurrency load-benchmarking.

---

## 🚀 Performance Benchmarks (Empirical Metrics)

Stress-tested under varying concurrent client loads using the included automated benchmarking tool (enchmark.go):

| Concurrent Clients | Total Msgs Sent | Broadcast Throughput | Fan-Out Throughput | Latency (P50) | Latency (P99) | Error Rate |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **50 Users** | 1,000 | **32,916 msgs/sec** | **935,124 deliveries/sec** | **630 µs** | 7.33 ms | **0.00%** |
| **200 Users** | 4,000 | **47,737 msgs/sec** | **526,858 deliveries/sec** | **2.35 ms** | 12.17 ms | **0.00%** |
| **500 Users** | 5,000 | **31,245 msgs/sec** | **211,345 deliveries/sec** | **10.07 ms** | 33.39 ms | **0.00%** |

- **Peak Broadcast Throughput**: **47,737 messages/sec**
- **Peak Fan-Out Delivery Rate**: **935,124 deliveries/sec**
- **Zero Lock Contention**: Achieved 0 deadlocks and 0 data races across thousands of concurrent RPC calls.

---

## 🛠️ Concurrency Redesign & Deadlock Resolution

### The Vulnerability: Synchronous Lock-Held Dispatch
In the naive implementation, client channels were **unbuffered** (make(chan string)) and delivery was executed **inside the mutex critical section**:
`go
// ❌ FLAGGED DEADLOCK VULNERABILITY:
func (c *ChatRoom) BroadcastMessage(...) {
    c.mu.Lock()
    defer c.mu.Unlock()
    for _, ch := range c.clients {
        ch <- message // BLOCKS until client executes ListenForMessages()!
    }
}
`
**The Deadlock Cycle**:
1. When a client registered or broadcasted, c.mu.Lock() was held while sending to an unbuffered channel.
2. A recipient client calling ListenForMessages() needed c.mu.Lock() to retrieve its channel.
3. This created a circular wait: the sender held the mutex waiting on the channel read, while the receiver held up the channel read waiting on the mutex.
4. Any client with latency or network pause caused head-of-line blocking that completely froze all other connected users.

### The Solution: Decoupled Non-Blocking Fan-Out
1. **Buffered Client Ring-Channels**: Upgraded channels to buffered queues (ClientChannelBuffer = 512) to absorb message bursts and network jitter.
2. **Read/Write Mutex Optimization**: Replaced sync.Mutex with sync.RWMutex, allowing simultaneous channel lookups without locking out concurrent readers.
3. **Out-of-Lock Snapshot Delivery**: Recipient channel slices are snapshotted quickly under c.mu.RLock(), and the lock is **released immediately** before any message delivery begins.
4. **Non-Blocking Select Fan-Out**:
   `go
   // ✅ DEADLOCK-FREE NON-BLOCKING FAN-OUT:
   for _, ch := range targets {
       select {
       case ch <- message:
       default:
           // Channel full: dropped for slow client to prevent system-wide lag
       }
   }
   `
5. **Clean Unregistration**: Added UnregisterClient() to gracefully close channels and clean up client registrations upon disconnect (/quit).

---

## 📐 System Architecture

`mermaid
sequenceDiagram
    autonumber
    actor Alice as Client A (Alice)
    participant Server as RPC Coordinator (ChatRoom)
    actor Bob as Client B (Bob)

    Note over Server: Server listening on 0.0.0.0:4488

    Alice->>Server: RegisterClient("Alice")
    Server-->>Alice: "Alice registered successfully!"
    Alice->>Server: go ListenForMessages("Alice") [Awaiting Buffer]

    Bob->>Server: RegisterClient("Bob")
    Note over Server: Non-blocking push to Alice's buffer
    Server-->>Bob: "Bob registered successfully!"
    Server-->>Alice: "Bob joined the chat!"
    Bob->>Server: go ListenForMessages("Bob") [Awaiting Buffer]

    Bob->>Server: BroadcastMessage("Bob", "Hello Everyone!")
    Note over Server: Snapshot channels & dispatch outside lock
    Server-->>Bob: "Message broadcasted"
    Server-->>Alice: "Bob says: 'Hello Everyone!'"
`

---

## 📂 Repository Structure

`	ext
EventDriven_RCPchatroom/
├── assets/
│   └── demo.png            # Multi-client execution screenshot
├── chatroom/
│   ├── commons/
│   │   └── shared.go       # RPC payload structs & fixed address config
│   ├── room.go             # ChatRoom business logic, thread-safe state & dispatcher
│   ├── server.go           # Server executable & RPC listener entry point
│   ├── client.go           # Interactive terminal client with background listener
│   ├── benchmark.go        # Multi-client concurrent load tester & latency profiler
│   ├── chatroom_test.go    # Concurrency and unregistration unit tests
│   ├── go.mod              # Go module definition (1.23)
│   └── test.sh             # Linux multi-terminal launcher
├── .gitignore
├── LICENSE
└── README.md
`

---

## ⚡ Running the Benchmark Suite

To reproduce the benchmark figures locally:

`ash
cd chatroom

# Run quick benchmark (100 concurrent clients):
go run benchmark.go room.go -quick

# Run full multi-tier benchmark (50, 200, 500 clients):
go run benchmark.go room.go

# Run concurrency unit tests:
go test -v room.go chatroom_test.go
`

---

## 🖥️ Running the Application

### 1. Start the Server
`ash
cd chatroom
go run server.go room.go
`
Output:
`	ext
Server is listening on 0.0.0.0:4488
`

### 2. Connect Interactive Clients
Open separate terminal tabs:
`ash
cd chatroom
go run client.go
`
- Enter your nickname when prompted.
- Type any message and press Enter to broadcast.
- Type /quit to unregister and disconnect cleanly.

---

## 👤 Author

**Mohamed Rslan**
- GitHub: [@Mohamed-Rslan](https://github.com/Mohamed-Rslan)
- Email: [m.rslan4488@gmail.com](mailto:m.rslan4488@gmail.com)

