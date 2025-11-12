# The Choke Algorithm

BitTorrent works on a P2P network, hence there is no central resource allocation unit.

## The Problem

How would the peers ensure:
1. Maximum download speed
2. Prevent anyone from abusing the network

A peer would naturally try to download from whoever it can.

## Introduction to the Choke Algorithm

The Choke algorithm was introduced to guarantee a reasonable upload and download reciprocation.

The choke algorithm is a variant of "tit-for-tat" algorithm.

## Free Rider Problem

Free-riders are peers that never upload, but always download. They should be penalized.

Hence, our criteria to choose a peer to send our content to cannot be "simple" — it should be based on reciprocation.

## Choking and Interested

### Choking

**Choking** is a temporary refusal to upload.

We say that peer A has choked peer B because peer B is unable to download file from A, but peer A can download from peer B.

### Why Choking is Necessary

Choking is necessary because:
1. TCP congestion when we send across many connections at once.
2. Prevent network abuse and starvation.

### Choking and Unchoking

Choking and Unchoking aren't perpetual; rather, it is periodic.

The upload happens only until a peer is unchoked.

### Interested Peer

**Interested peer** is someone who wants a piece that you have.

For example: A wants a piece p1 that is there with B, hence we say that A is **INTERESTED** in peer B.

## How to Find Peers to Unchoke

### Peer Selection in the Swarm

There may be thousands of nodes in the swarm, but a few of them are randomly picked by tracker as a peer for each.

But, how would we find peers to choke and unchoke?

### Criteria: Download Rate and Reciprocity

The selection is based on:
- **Download rate** → **Reciprocity**

**Core Principle**: Any peer will upload to peers who give the best download rate.

This approach:
- Encourages peers to let others download
- Prohibits free riders that never upload

### Reciprocity-Based Unchoking

We are prioritizing to **unchoke** peers who let us download the pieces recently.

**Reciprocity** - returning a favor.

### Example

A is choked by B, i.e., it cannot get any piece from B.

B wanted a piece that A had, and since A gave that piece, B **unchokes** A, allowing it to now download from B.

## Choke Algorithm for Leecher

### When the Algorithm is Called

When in leecher state, the choke algorithm is called:
- Every 10 seconds
- Every time a peer leaves a peerset
- Every time an unchoked peer becomes interested or not

### Regular Unchoke

1. Every 10 seconds, peer A orders the interested remote peers by their download rate to A, and the fastest 3 are **unchoked** (Regular Unchoke).

2. For regular unchoke, the peers are ordered by their download rate to local peer and who have sent at least one block in the last 30 seconds. This guarantees only active peers are unchoked.

### Optimistic Unchoke

3. Every 30 seconds, one additional interested peer is unchoked at random (no need of reciprocation). This is done to promote fairness to new peers.

#### Advantages of Optimistic Unchoke

- Evaluates download capacity of new peers
- Bootstrap new peers who do not have any piece to share, by giving them the first piece

#### Optimistic Unchoke Logic

4. If the optimistic unchoked peer is from the 3 fastest peers, another peer is chosen for an unchoke:
   - If peer was also interested, the round completes
   - If peer is not interested, still it is unchoked and we continue to unchoke other peers optimistically

> Note: We would have more than 4 unchokes, but at max 4 interested unchokes.

## Promoting Good Behavior

The choking algorithm boosts "good" behavior:
- If upload rate of A is high, more peers will unchoke it, giving A a faster download.

## Free Rider Penalization

Free riders never upload; they only download. Hence, when other peers are choosing whom to unchoke, the free rider will be last in the list because of sorting criteria (ordered by download rate to A).

The only hope for a free rider is **Optimistic Unchoke**.

## Anti-snubbing

There is a possibility that a peer is **CHOKED** by all others.

How would it proceed?

- **Optimistic unchoke** is infrequent for a peer who has not sent anything in the last 60 seconds.
- The choked peer retaliates and refuses to upload to the peer that choked it.
- This behavior increases optimistic unchokes in the network.

## Choke Algorithm for Seeder

### When the Algorithm is Called

When in seeder state, the choke algorithm is called:
- Every 10 seconds or every time a peer leaves a peerset
- Every time an unchoked peer becomes interested / not interested

### Algorithm Behavior

1. The algorithm orders the peers according to the time they were last unchoked or that have pending request for blocks.

Higher upload rate is given a priority.

2. The other peers (never unchoked) are ordered by upload rates.

3. **For first 20 seconds:**
   - Unchoke first 3 peers (Seed kept unchoked)
   - And unchokes one peer at random (Seed random Unchoked)

4. **For the next 10 seconds:**
   - Unchokes the first 4 peers (Seed kept unchoked)

### Important Notes

Seeder is not unchoking based on upload rate; instead, it is using the time they were last unchoked peers in active peer set are changed regularly random peer taking a slot from too ordered peers.
