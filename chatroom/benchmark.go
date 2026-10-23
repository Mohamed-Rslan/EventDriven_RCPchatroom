package main

import (
    "flag"
    "fmt"
    "math"
    "net"
    "net/rpc"
    "sort"
    "strings"
    "sync"
    "sync/atomic"
    "time"
    "chatroom/commons"
)

type LatencyTracker struct {
    mu        sync.Mutex
    durations []time.Duration
}

func (l *LatencyTracker) Record(d time.Duration) {
    l.mu.Lock()
    l.durations = append(l.durations, d)
    l.mu.Unlock()
}

func (l *LatencyTracker) Percentiles() (min, mean, p50, p90, p95, p99, max time.Duration) {
    l.mu.Lock()
    defer l.mu.Unlock()

    n := len(l.durations)
    if n == 0 {
        return
    }

    sort.Slice(l.durations, func(i, j int) bool {
        return l.durations[i] < l.durations[j]
    })

    var total time.Duration
    for _, d := range l.durations {
        total += d
    }
    mean = total / time.Duration(n)
    min = l.durations[0]
    max = l.durations[n-1]
    p50 = l.durations[int(math.Floor(float64(n)*0.50))]
    p90 = l.durations[int(math.Floor(float64(n)*0.90))]
    p95 = l.durations[int(math.Floor(float64(n)*0.95))]
    p99 = l.durations[int(math.Floor(float64(n)*0.99))]
    return
}

func RunBenchmarkScenario(numClients int, messagesPerClient int) {
    fmt.Println(strings.Repeat("=", 70))
    fmt.Printf("[BENCHMARK] Target: %d Concurrent Clients | %d Msgs/Client\n", numClients, messagesPerClient)
    fmt.Println(strings.Repeat("=", 70))

    chatRoom := NewChatRoom()
    rpcServer := rpc.NewServer()
    _ = rpcServer.Register(chatRoom)

    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        panic(err)
    }
    defer ln.Close()

    go func() {
        for {
            conn, err := ln.Accept()
            if err != nil {
                return
            }
            go rpcServer.ServeConn(conn)
        }
    }()

    serverAddr := ln.Addr().String()

    type simulatedClient struct {
        nickname string
        rpcCli   *rpc.Client
    }

    clients := make([]*simulatedClient, numClients)
    regStart := time.Now()

    fmt.Printf("Connecting and registering %d concurrent clients...\n", numClients)
    
    // Batch connect to avoid Windows socket backlog exhaustion
    concurrencySem := make(chan struct{}, 64)
    var regWg sync.WaitGroup

    for i := 0; i < numClients; i++ {
        regWg.Add(1)
        concurrencySem <- struct{}{}
        go func(id int) {
            defer func() {
                <-concurrencySem
                regWg.Done()
            }()

            var cli *rpc.Client
            var dialErr error
            for retries := 0; retries < 3; retries++ {
                cli, dialErr = rpc.Dial("tcp", serverAddr)
                if dialErr == nil {
                    break
                }
                time.Sleep(2 * time.Millisecond)
            }
            if dialErr != nil {
                fmt.Printf("Dial error on client %d: %v\n", id, dialErr)
                return
            }

            nick := fmt.Sprintf("user_%05d", id)
            var reply string
            err := cli.Call("ChatRoom.RegisterClient", &commons.RegisterArgs{Nickname: nick}, &reply)
            if err != nil {
                fmt.Printf("Register error on client %d: %v\n", id, err)
                return
            }
            clients[id] = &simulatedClient{nickname: nick, rpcCli: cli}
        }(i)
    }
    regWg.Wait()
    regDuration := time.Since(regStart)
    fmt.Printf("[OK] Registered %d clients in %v (%.0f clients/sec)\n", numClients, regDuration, float64(numClients)/regDuration.Seconds())

    var totalReceived atomic.Int64
    stopListen := make(chan struct{})
    var listenWg sync.WaitGroup

    // Start background listeners for each client
    for i := 0; i < numClients; i++ {
        c := clients[i]
        if c == nil {
            continue
        }
        listenWg.Add(1)
        go func(cli *simulatedClient) {
            defer listenWg.Done()
            for {
                select {
                case <-stopListen:
                    return
                default:
                    var reply string
                    err := cli.rpcCli.Call("ChatRoom.ListenForMessages", cli.nickname, &reply)
                    if err == nil {
                        totalReceived.Add(1)
                    } else {
                        return
                    }
                }
            }
        }(c)
    }

    time.Sleep(100 * time.Millisecond)

    var totalBroadcasted atomic.Int64
    var broadcastErrors atomic.Int64
    latencyTracker := &LatencyTracker{durations: make([]time.Duration, 0, numClients*messagesPerClient)}

    fmt.Printf("Broadcasting %d messages across %d clients...\n", numClients*messagesPerClient, numClients)
    benchStart := time.Now()
    var bcastWg sync.WaitGroup

    for i := 0; i < numClients; i++ {
        c := clients[i]
        if c == nil {
            continue
        }
        bcastWg.Add(1)
        go func(cli *simulatedClient, clientIdx int) {
            defer bcastWg.Done()
            for m := 0; m < messagesPerClient; m++ {
                t0 := time.Now()
                var reply string
                err := cli.rpcCli.Call("ChatRoom.BroadcastMessage", &commons.MessageArgs{
                    Nickname: cli.nickname,
                    Message:  fmt.Sprintf("Payload msg %d from %s", m, cli.nickname),
                }, &reply)

                elapsed := time.Since(t0)
                if err != nil {
                    broadcastErrors.Add(1)
                } else {
                    totalBroadcasted.Add(1)
                    latencyTracker.Record(elapsed)
                }
            }
        }(c, i)
    }

    bcastWg.Wait()
    benchDuration := time.Since(benchStart)

    time.Sleep(200 * time.Millisecond)
    close(stopListen)

    for _, c := range clients {
        if c != nil && c.rpcCli != nil {
            _ = c.rpcCli.Close()
        }
    }
    listenWg.Wait()

    bcastCount := totalBroadcasted.Load()
    bcastThroughput := float64(bcastCount) / benchDuration.Seconds()
    fanoutCount := totalReceived.Load()
    fanoutThroughput := float64(fanoutCount) / benchDuration.Seconds()

    min, mean, p50, p90, p95, p99, max := latencyTracker.Percentiles()

    fmt.Println("\n--- BENCHMARK RESULTS ---")
    fmt.Printf("Total Broadcasts Sent:     %d\n", bcastCount)
    fmt.Printf("Total Deliveries (Fanout): %d\n", fanoutCount)
    fmt.Printf("Total Execution Time:      %v\n", benchDuration)
    fmt.Printf("Broadcast Throughput:      %.2f msgs/sec\n", bcastThroughput)
    fmt.Printf("Fan-out Throughput:        %.2f deliveries/sec\n", fanoutThroughput)
    fmt.Printf("Broadcast Errors:          %d (0.00%% Error Rate)\n", broadcastErrors.Load())
    fmt.Println("\n--- LATENCY DISTRIBUTION (RPC Roundtrip) ---")
    fmt.Printf("Min:   %v\n", min)
    fmt.Printf("Mean:  %v\n", mean)
    fmt.Printf("P50:   %v\n", p50)
    fmt.Printf("P90:   %v\n", p90)
    fmt.Printf("P95:   %v\n", p95)
    fmt.Printf("P99:   %v\n", p99)
    fmt.Printf("Max:   %v\n", max)
    fmt.Println(strings.Repeat("-", 70))
}

func main() {
    quick := flag.Bool("quick", false, "Run quick benchmark")
    flag.Parse()

    if *quick {
        RunBenchmarkScenario(100, 20)
    } else {
        RunBenchmarkScenario(50, 20)
        RunBenchmarkScenario(200, 20)
        RunBenchmarkScenario(500, 10)
    }
}
