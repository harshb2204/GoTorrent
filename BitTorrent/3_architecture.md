# The BitTorrent Architecture

## BitTorrent Architecture Entities

The BitTorrent architecture consists of the following entities:

1. **.torrent file**
2. **Tracker**
3. **Seeders**
4. **Leechers**

The tracker acts as a central coordinator connecting seeders and leechers. When you (the leecher) want to download a file, you connect to the tracker which provides information about available seeders who have the complete file or parts of it.

## Pieces

### File Splitting

The original file that is to be shared in the network is split into **equal sized pieces**.

- Piece size ranges from **256 KB to 1 MB**
- Each file is divided into multiple pieces (p1, p2, p3, etc.)

### SHA-1 Hashing

The **SHA-1 hash** of each piece is added in the .torrent file under the `pieces` attribute.

### Piece Verification

- Each piece is fetched by its **SHA-1 hash** (from the .torrent file)
- Once a piece is downloaded, it is checked for corruption using its very own **SHA-1 hash**
- This ensures data integrity throughout the download process

## Swarm and Piece Propagation

Once a peer downloads a piece, it announces availability to other peers in the swarm. This allows peers to download pieces from one another (not just from the original seeder), accelerating distribution as more peers join.

## Torrent File (Overview)

- **Metafile (not data)**: Contains static information about the content such as file name(s), total size, and the list of piece hashes. It does not contain the actual content.
- **announce URL**: Includes a tracker URL under the `announce` field. The tracker tells clients where to find peers for this torrent.
- **Unique identity**: Each torrent is uniquely identified by its **info hash** (a SHA-1 over the `info` dictionary).
- **How it’s obtained**: `.torrent` files are typically downloaded over regular HTTP/HTTPS or shared via magnet links.

## Tracker

Tracker is the only central entity for a torrent.

### Core responsibilities

1. Keep track of peers who hold the file (seeders)
2. Keep track of peers who are downloading (leechers)
3. Help a peer discover other peers to download content from

> Note: A tracker does not download or transfer any file. It only maintains information about peers and distribution.

### Tracker is just a simple HTTP server that

- Hands out peer information to the network over HTTP
- Periodically collects statistics/announces from peers
![](/BitTorrent/diagrams/overview.png)
- You have .torrent file and extract some information and go to the tracker and say that you want to be part of the network.
- Tracker would send information about 50 peers. Given that you have information you will talk to each one of them to download the pieces then broadcast the information in the network. 

### Peer set and periodic reporting

- Once a machine receives a list of about 50 peers from the tracker, it adds them to its peer set.
- Every peer reports its state to the tracker roughly every 30 minutes, including:
  - The amount of bytes it has uploaded since it joined the torrent
  - The amount of bytes it has downloaded since it joined the torrent 

### Peer Set Management

- **Peer set replenishment**: If the number of peers in the peerset drops below 20, the peer will reach out to the tracker to get a new list of peers.

- **Maximum peerset = 80**
- **Maximum connections = 80**
  - For download, max 40 connections
  - For upload, other 40 connections

> Note: The numbers and limits are tunable.

- **Piece awareness**: Each peer knows which pieces each peer in its peerset has through gossip.



## Overview Of Bittorrent

![](/BitTorrent/diagrams/overview1.png)
- You are a peer and have a torrent file and want to download something.
- Someone first has to upload the original file in the torrent network. (First seeder). First seeder would create a .torrent file.
- Torrent server syncs it to its own search engine.
- You as a peer would go to the search engine and get the torrent.
- Now your job is to go to a tracker and ask for the peer list. Once you get the peer list you would then talk to each one of them to request for a download.
- Whoever has a piece of it can share you that piece. Once you have that piece you would trade the piece with others in your network.

## Seeder and Leecher

- **Seeder** is a peer that has the entire file.
- **Leecher** is a peer that is downloading the file.
- A leecher can download file from seeder or peer leecher.
- Once a leecher has the entire file, it becomes a seeder.
