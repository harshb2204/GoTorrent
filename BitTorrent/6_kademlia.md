# Kademlia - a pure P2P Distributed Hash Table

*   To get information about peers, a node in the BitTorrent network talks to **Tracker**.
*   Having a central entity is still prone to attacks and failures
*   So, can we do a pure p2p network, without Tracker?

## Distributed Hash Table

Say, we have a gigantic set of kv pairs that one node cannot store or handle. Hence we have to **distribute**. Hence it is called a **Distributed** Hash Table.


- We have a lot of nodes in our network which are storing some bunch of kv pairs. There is no central entity.
- Lets say I want to get a particular kv pair request can come to any node, and it needs to know who to talk to, to reach the node who holds the kv pair. This is the overlay network.

## Key Questions

1.  How do we distribute?
2.  How would a node know how to find a key?
3.  How to gracefully handle nodes joining / leaving?

## Representation

1.  **Every node (machine) participating gets a unique 160b (20B) ID.**

2.  **The unique ID can be**
    *   **- explicitly assigned, for P2P**
    *   **- implicitly derived**
        *   Node IP → f → h

3.  **The data that is stored across the network is also hashed and identified by 160b ID**
    *   **KV pair**
    *   key → f → h

*   This is a generic DHT, nothing specific to BitTorrent. In the context of BitTorrent, the only thing that changes is the kind of information (reachable peers) stored on the node.

## Ownership

*   key k₁ → h_k₁
*   Node N₁ → h_N₁
*   The node that is **closest** to the key, owns the key. * not a ring
*   kᵢ ∈ Nⱼ | d(h_kᵢ, h_Nⱼ) is minimum for ∀j

## Distance metric

In order to find the "closest" node that quantifies the closeness → For any non-euclidean geometry

### Requirement from a distance metric

1.  **d(x,x) = 0 ∀x** — distance to self = 0
2.  **d(x,y) > 0 if x ≠ y ∀x,y** — distance to others is +ve
3.  **d(x,y) + d(y,z) ≥ d(x,z)** — triangle inequality

![](/BitTorrent/diagrams/geometry.png)

For two nodes / keys in our Kademlia distribution, the distance metric is:

**d(x,y) = x ⊕ y**

Bitwise XOR of 160-bit IDs, interpreted as an integer.

This metric satisfies all three requirements:

1.  **d(x,x) = x ⊕ x = 0**
2.  **d(x,y) = x ⊕ y > 0** (if x ≠ y)
3.  **d(x,y) + d(y,z)**
    *   = (x ⊕ y) ⊕ (y ⊕ z)
    *   = x ⊕ z = d(x,z)

## Visualizing Distance

For simplification, say we work with 4-bit IDs (both nodes and keys).

| Node / Key | Decimal ID | Bit Representation |
| --- | --- | --- |
| N₁ | 15 | 1111 |
| k_A | 6 | 0110 |
| N₂ | 5 | 0101 |
| k_B | 13 | 1101 |

### Example

*   **d(k_A, N₁) = 0110 ⊕ 1111 = 1001 = 9**
*   **d(k_A, N₂) = 0110 ⊕ 0101 = 0011 = 3**

Hence, key k_A is owned by node N₂ (the closer node).

### Prefix intuition

*   The bits that are the same between two IDs XOR to 0, so they do not contribute to the distance.
*   Therefore, IDs sharing a longer common prefix are “closer” under XOR distance.
*   We can visualize the ID space as a binary trie; paths that share longer prefixes meet deeper in the trie and thus have smaller XOR distance.


Visualization as a trie
![](/BitTorrent/diagrams/tree.png)

*   Instead of creating a complete binary tree, we create the paths as needed.
*   Instead of creating complete path we carve it till it minimally disambiguates.

| Node / Key | Decimal ID | Bit Representation |
| --- | --- | --- |
| N₁ | 15 | 1111 |
| k_A | 6 | 0110 |
| N₂ | 5 | 0101 |
| k_B | 13 | 1101 |
| k_C | 8 | 1000 |
| k_D | 9 | 1001 |
| N₃ | 1 | 0001 |

## Routing

Given that there is no central entity to hold the addresses of all the nodes, how would one node access the KV on the other?

**Problem:** A client requests GET K₅ from N₁, but K₅ V₅ is stored on N₂. How does N₁ find N₂?

**Solution approach:**

Every node in the network would need to keep track of a few nodes, and hope they keep track of others, and so on. Eventually we would have covered the entire network.

Peer nodes that each node keeps track of cannot be random, as we need guaranteed convergence **quickly**.

So, what should be our routing strategy, that ensures...

### Core routing idea

*   Every node keeps at least one contact in each subtree that it is not part of.
*   For a node like N₁ (with prefix 0101...), its routing table should have contacts in subtrees rooted at the prefixes `1`, `00`, `011`, `0101`, etc.
*   This ensures that any lookup can always progress toward the target ID by following a contact that reduces the XOR distance.


If every node in the network keeps track of at least one node in each subtree, the lookup converges to the desired node in **O(log n)** hops.

### Example lookup

* **Goal:** N₁ (`0000`) wants to reach N₂ (`1111`) without a direct connection.
* It leverages intermediate nodes from its routing table:
    * N₁ (`0000`) → N_A (`1000`)
    * N_A (`1000`) → N_B (`1101`)
    * N_B (`1101`) → N₂ (`1111`)
* Each node contributes a contact from the appropriate subtree (`1`, `11`, `111`...), steadily decreasing XOR distance.

Thus, the XOR-based distance metric guarantees convergence—each hop always moves closer to the target without digressing.

Thus each node only has to keep track of a small subset of nodes and the routing takes care of converging to the target node.

Communication happens over UDP and routing table holds `node id → <ip, udp port>`.

As the routing converges when every node has a few contacts in every subtree that it is not part of, the problem statement reduces to making this structure fault tolerant.

### K-buckets

Every node, for each subtree, holds `k` entries:

| Subtree prefix | Node ID | IP | UDP port |
| --- | --- | --- | --- |
| `1` | ... | ... | ... |
| `00` | ... | ... | ... |
| `011` | ... | ... | ... |
| `0101` | ... | ... | ... |

Each k-bucket is sorted by **time last seen**, with the most recently seen contact at the tail. A typical `k` is 20, meaning each node maintains up to 20 contacts per subtree.

### Updating the routing table

When a node receives any message from another node, it updates the appropriate k-bucket with that node’s contact information.

1. Entries are always appended at the tail (most recent end).
2. If the k-bucket already contains the node, the entry is moved to the tail.

If the k-bucket is full:

* The node pings the least-recently seen contact (at the head).
* If that contact does not respond, it is evicted and the new node is added to the tail.
* If it does respond, the new node is discarded and the existing contact is moved to the tail.

It is observed that if a node has been online for a long time, it is likely to remain online; the k-bucket policy exploits this longevity.

## Communication interface

Every node participating in Kademlia exposes four RPCs:

* **PING** — Probe a node to check if it is online.
* **FIND_NODE** — Return `<ip, port, nodeId>` tuples for the `k` nodes known that are closest to the requested node ID.
* **FIND_VALUE** — Same as `FIND_NODE`, but if the contacted node holds the value for the requested key, it returns the value instead.
* **STORE** — Instruct a node to persist a `<key, value>` pair.


Notes:

* Intermediate nodes do **not** forward lookup requests; they simply return the contacts that move us closer to the target.
* The lookup continues iteratively until the target node or value is reached and the action completed.

To store a `<key, value>` pair, a node first locates the `k` closest nodes to the key (via `FIND_NODE`) and then sends each of them a `STORE` RPC.

Each node periodically republishes the keys it holds to keep data alive in the network:

* The original publisher republishes every 24 hours.
* Nodes store values with a 24-hour expiration, refreshing whenever they republish or receive refreshed data.

Because use cases vary, `STORE` implementations differ (e.g., BitTorrent vs digital certificates):

* Single copy vs multiple replicas
* Expiration vs no expiration
* Read/write responsibilities

### Performance optimization

Cache `<key, value>` pairs along the lookup path (with LRU eviction). If a node goes down, neighboring nodes that recently served the data likely still have it cached, maintaining availability.


