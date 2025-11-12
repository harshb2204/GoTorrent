# The Torrent File

The Torrent file is the session of transfer of a single content to the set of peers.

* Each torrent is independent

For a user to download anything from the network, it needs a Torrent file of the content.

## Lifecycle of a torrent

Torrent is alive as long as there is *at least one seeder*.

There is no incentive for anyone to join a torrent and become a seeder.

A user downloads the torrent from websites via normal http request

User uses the torrent file and a client to download the file and upon completion, it can discard the torrent file.

## What torrent file holds?

The .torrent file holds meta information about the content, like:

1. **announce**: the announce URL of tracker
2. **created by**: name and version of program who created it
3. **creation date**: creation time of torrent in UNIX epoch
4. **encoding**: encoding of strings as part of `info` dictionary
5. **comment**: Some additional comment about author/content
6. **info**: a dictionary that describes file(s) of the torrent

### The info Dictionary (The Heart of Torrent Metadata)

This dictionary is crucial because it defines the structure and integrity of the content.

Also — its bencoded form's SHA1 hash is what gives every torrent its unique "infohash", used to identify it in the network.

The info dictionary can be in two formats:

#### 1. Single file format
- **name**: filename of content
- **length**: file size in bytes
- **md5sum**: md5 of the file

#### 2. Multi-file format
- **name**: name of the directory
- **files**: list of dictionaries, one for each file
  - **length**: length of the file
  - **md5sum**: md5sum of the file
  - **path**: list of string representing

#### Common Fields

The `info` dictionary also contains these crucial fields:

| Key | Description |
|-----|-------------|
| **piece length** | Number of bytes in each "piece" of the file(s). Typical values: 256 KB, 512 KB, 1 MB |
| **pieces** | A concatenation of 20-byte SHA1 hashes. Each hash corresponds to one "piece" of the data. |
| **private** (optional) | If 1, it restricts the torrent to specific trackers (no DHT or peer exchange). |

![](/BitTorrent/diagrams/infodictionary.png)

## Torrent Fileformat - Bencoding

Torrent files are **Bencoded**, and to extract the above fields we would need to parse the torrent file (Bencoded).

![](/BitTorrent/diagrams/bencoding.png)

### Bencoding Specification

Bencoding supports: **strings, lists, integers, and dictionaries**

#### 1. Strings
- **Format**: `<length>:<string>`
- **Example**: `harsh` → `5:harsh`

#### 2. Integers
- **Format**: `i<integer>e`
- **Example**: `10` → `i10e`

#### 3. List
- **Format**: `l<bencoded values>e`
- **Example**: `["a", "b", 1]` → `l1:a1:bi1ee`

#### 4. Dictionary
- **Format**: `d<bencoded key><bencoded value>e`
- **Example**: `{"name": "harsh", "age": 21}` → `d4:name5:harsh3:agei21ee`
