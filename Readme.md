# Load Balancing in Go

A load balancer is a tool commonly used to coordinate the volume of traffic between available servers, ensuring high availability and reliability by redistributing the load.

---

## 🚦 Load Balancing Techniques

A load balancer sits in front of a group of backend servers, referred to as a **server pool**. The distribution of traffic is performed based on specific **load balancing algorithms** which are responsible for choosing the most appropriate backend for each incoming request.

---

## 🔄 Round Robin Algorithm

The **Round Robin** algorithm distributes the load **equally** among the servers in the server pool using a simple rotation.

- **Logic:** It moves through the list of servers sequentially ($1 \rightarrow 2 \rightarrow 3 \rightarrow 1$).
- **Check:** It performs a basic health check to ensure a server is alive before forwarding the request.

---

## ⚖️ Weighted Round Robin

This algorithm forwards requests based on the **power** or capacity of each individual server.

- **Scenario:** If Server A has twice the processing power of Server B, Server A will be assigned a higher "weight."
- **Result:** For every 1 request Server B handles, Server A will handle 2 requests. This prevents weaker servers from being overwhelmed while maximizing the utility of more powerful hardware.

---

## 📉 Least Connections Method

Unlike the static rotation of Round Robin, this method is **dynamic**. it considers the current workload of each server.

- **Logic:** The load balancer tracks the number of active connections or requests each server is currently handling.
- **Selection:** New requests are sent to the server with the **least** active connections.
- **Best Use Case:** This is ideal for requests that take varying amounts of time to process, preventing "clumping" on a single server.

---

## 💡 Implementation Note (Go)

In Go, these techniques are typically implemented using:

- **`sync/atomic`**: For thread-safe counter management in Round Robin.
- **`httputil.ReverseProxy`**: To handle the actual forwarding of the request/response.
- **`sync.RWMutex`**: To safely manage the "Alive" status of the server pool across multiple goroutines.
