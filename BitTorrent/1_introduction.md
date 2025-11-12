# Introduction To BitTorrent

 BitTorrent is a peer to peer protocol that makes distribution of large files - 
 1. Easier
 2. faster
 3. Efficient

## Classic Download & need of BitTorrent

Client requests for the file from the server, and the server responds. Things become interesting when there are large number of clients or a larger file to download.

### Problems:
- Server's bandwidth is limited, so, more clients will slow things down
- Speed of data transfer is limited by the upload capacity

### Example:
If user B's upload speed is 60Mbps, then no matter the download speed of A, the overall download speed cannot go beyond 60Mbps. Can we do better?

## Peer to Peer Networks
![](/BitTorrent/diagrams/p2p.png)

Each party has the same capabilities, and can initiate conversation with other.

### Key highlight of P2P: robustness

Even if you remove one node from the network, there would not be any impact on the service.

**No single point of failure!**

### Central Entity in P2P

![](/BitTorrent/diagrams/cp2p.png)

There also may be a central entity to provide some functionalities:

- The peer nodes are still equal and would still communicate with each other directly
- But some info can be provided by the central entity

**Note:** the network and its services will be affected when the central entity goes down.

Hence, this setup is more vulnerable to failures.

## Core Idea of BitTorrent


Download the file from multiple machines, concurrently

### Benefits:
- faster downloads
- upload load is distributed b/w peers
- better utilization of download capacity
  - 100mbps download
  - 60 Mbps upload
- large number of downloaders would put only a little extra load
- breaking file into smaller chunks would boost concurrency

## Simplified download flow

When a user wants to download a file, it sniffs around the network to find peers having pieces of it. User then downloads different pieces from different users concurrently → faster download and better utilization of download capacity
![](/BitTorrent/diagrams/p2p1.png)

## 1. Pieces and Blocks

![](/BitTorrent/diagrams/pieceandblocks.png)

A file that is shared in the BitTorrent network is split into **pieces** and each piece is further split into **blocks**.

In one transfer, a **block** is transferred but a **piece** is served by a peer.

**\* a piece cannot be served if any of the blocks is missing**

## 2. Peer Set

Each peer maintains a list of peers that it can send pieces to and this is called its **peer set**.

- peerset(A) = {C, E}
- peerset(E) = {A, B, C}

## 3. Active Peer Set

A peer can only send data to a subset of its peer set and this is called an **active peer set**.

- Active peer set(A) = {C}
- active peer set(E) = {A, B}

## 4. Seeders and leechers

A peer can be a **seeder** or a **leecher**.

- **Leecher**: when a peer is downloading
- **Seeder**: when a peer has all the pieces of the content

Large number of seeders would lead to a faster download speed, as we can pull from multiple seeders quickly.

if leechers >> seeders, download speed could take a hit.

### BitTorrent is popular friendly

The new and popular files will have a lot of seeders, hence it would be downloaded faster. Old or unpopular files will have few seeders, hence a slower download.

## Applications of BitTorrent

1. Downloading Linux Distributions - faster than FTP and HTTP
   - and large softwares, movies, games, etc
2. Sending patches to users (eg: security patches)
3. Facebook uses this to power their massive deployments
   - deploying artifacts across servers