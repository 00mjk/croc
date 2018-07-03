
<p align="center">
<img
    src="https://user-images.githubusercontent.com/6550035/31846899-2b8a7034-b5cf-11e7-9643-afe552226c59.png"
    width="100%" border="0" alt="croc">
<br>
<a href="https://github.com/schollz/croc/releases/latest"><img src="https://img.shields.io/badge/version-3.0.0-brightgreen.svg?style=flat-square" alt="Version"></a>
<a href="https://saythanks.io/to/schollz"><img src="https://img.shields.io/badge/Say%20Thanks-!-yellow.svg?style=flat-square" alt="Go Report Card"></a>
</p>


<p align="center">Easily and securely transfer stuff from one computer to another.</p>

*croc* allows any two computers to directly and securely transfer files and folders. When sending a file, *croc* generates a random code phrase which must be shared with the recipient so they can receive the file. The code phrase encrypts all data and metadata and also serves to authorize the connection between the two computers in a intermediary relay. The relay connects the TCP ports between the two computers and does not store any information (and all information passing through it is encrypted). 

**New version released Jly 3rd, 2018 - please upgrade if you are using the public relay.**

I hear you asking, *Why another open-source peer-to-peer file transfer utilities?* [There](https://github.com/cowbell/sharedrop) [are](https://github.com/webtorrent/instant.io) [great](https://github.com/kern/filepizza) [tools](https://github.com/warner/magic-wormhole) [that](https://github.com/zerotier/toss) [already](https://github.com/ipfs/go-ipfs) [do](https://github.com/zerotier/toss) [this](https://github.com/nils-werner/zget). But, after review, [I found it was useful to make another](https://schollz.github.io/sending-a-file/). Namely, *croc* has no dependencies (just [download a binary and run](https://github.com/schollz/croc/releases/latest)), it works on any operating system, and its blazingly fast because it does parallel transfer over multiple TCP ports.

# Example

<!-- _These two gifs should run in sync if you force-reload (Ctl+F5)_

**Sender:**

![send](https://raw.githubusercontent.com/schollz/croc/master/logo/sender2.gif)

**Receiver:**

![receive](https://raw.githubusercontent.com/schollz/croc/master/logo/receiver2.gif) -->


**Sender:**

```
$ croc send some-file-or-folder
Sending 4.4 MB file named 'some-file-or-folder'
Code is: cement-galaxy-alpha
On the other computer please run

croc cement-galaxy-alpha

Sending (->[1]63982)..
  89% |███████████████████████████████████     | [12s:1s]
Transfer complete.
```

**Receiver:**

```
$ croc
Enter receive code: cement-galaxy-alpha
Receiving file (4.4 MB) into: some-file-or-folder
ok? (y/N): y

Receiving (<-[1]63975)..
  97% |██████████████████████████████████████  | [13s:0s]
Received file written to some-file-or-folder
```

Note, by default, you don't need any arguments for receiving! This makes it possible for you to just double click the executable to run (nice for those of us that aren't computer wizards).

## Using *croc* in pipes

You can easily use *croc* in pipes when you need to send data through stdin or get data from stdout.

**Sender:**

```
$ cat some_file_or_folder | croc send
```

In this case *croc* will automatically use the stdin data and send and assign a filename like "croc-stdin-123456789".

**Receiver:**

```
$ croc --code code-phrase --yes --stdout receive
```

Here the reciever specified the code (`--code`) so it will not be prompted, and also specified `--yes` so the file will be automatically accepted. The output goes to stdout when flagged with `--stdout`.


# Install

[Download the latest release for your system](https://github.com/schollz/croc/releases/latest).

Or, you can [install Go](https://golang.org/dl/) and build from source with `go get github.com/schollz/croc`.



# How does it work? 

*croc* is similar to [magic-wormhole](https://github.com/warner/magic-wormhole#design) in spirit. Like *magic-wormhole*, *croc* generates a code phrase for you to share with your friend which allows secure end-to-end transferring of files and folders through a intermediary relay that connects the TCP ports between the two computers. Like *magic-wormhole*, security is enabled by performing password-authenticated key exchange (PAKE) with the weak code phrase to generate a session key on both machines without passing any private information between the two. The session key is then verified and used to encrypt the content with AES-256. If at any point the PAKE fails, an error will be reported and the file will not be transferred. More details on the PAKE transfer can be found at [github.com/schollz/pake](https://github.com/schollz/pake).

## Relay 

*croc* relies on a TCP relay to staple the parallel incoming and outgoing connections. The relay temporarily stores connection information and the encrypted meta information. The default uses a public relay at, `wss://croc3.schollz.com`. You can also run your own relay, it is very easy. On your server, `your-server.com`, just run

```
$ croc relay
```

Now, when you use *croc* to send and receive you should add `-server your-server.com` to use your relay server. Make sure to open up TCP ports (see `croc relay --help` for which ports to open).

# Contribute

I am awed by all the [great contributions](#acknowledgements) made! If you feel like contributing, in any way, by all means you can send an Issue, a PR, ask a question, or tweet me ([@yakczar](http://ctt.ec/Rq054)).



# Protocol

This is an outline of the protocol used here. The basic PAKE protocol is from [Dan Boneh and Victor Shoup's crypto book](https://crypto.stanford.edu/%7Edabo/cryptobook/BonehShoup_0_4.pdf) (pg 789, "PAKE2 protocol).

![Basic PAKE](https://camo.githubusercontent.com/b85a5f63469a2f986ce4d280862b46b00ff6605c/68747470733a2f2f692e696d6775722e636f6d2f73376f515756502e706e67)

1. **Sender** requests new channel and receives empty channel from **Relay**, or obtains the channel they request (or an error if it is already occupied).
2. **Sender** generates *u* using PAKE from secret *pw*.
3. **Sender** sends *u* to **Relay** and the type of curve being used. Returns error if channel is already occupied by sender, otherwise it uses it.
4. **Sender** communicates channel + secret *pw* to **Recipient** (human interaction).
5. **Recipient** connects to channel and receives UUID.
6. **Recipient** requests *u* from **Relay** using the channel. Returns error if it doesn't exist yet.
7. **Recipient** generates *v*, session key *k_B*, and hashed session key *H(k_B)* using PAKE from secret *pw*.
8. **Recipient** sends *v*, *H(H(k_B))* to **Relay**.
9. **Sender** requests *v*, *H(H(k_B))* from **Relay**.
10. **Sender** uses *v* to generate its session key *k_A* and *H(k_A)*, and checks *H(H(k_A))*==*H(H(k_B))*. **Sender** aborts here if it is incorrect.
11. **Sender** gives the **Relay** authentication *H(k_A)*.
12. **Recipient** requests *H(k_A)* from relay and checks against its own. If it doesn't match, then bail.
13. **Sender** connects to **Relay** tcp ports and identifies itself using channel+UUID.
14. **Sender** encrypts data with *k*.
15. **Recipient** connects to **Relay** tcp ports and identifies itself using channel+UUID.
16. **Relay** realizes it has both recipient and sender for the same channel so it staples their connections. Sets *stapled* to `true`.
17. **Sender** asks **Relay** whether connections are stapled.
18. **Sender** sends data over TCP.
19. **Recipient** closes relay when finished. Anyone participating in the channel can close the relay at any time. Any of the routes except the first ones will return errors if stuff doesn't exist.


### Conditions of state

The websocket implementation means that each client and relay follows their specific state machine conditions.

#### Sender

*Initialize*

- Requests to join.

*Does X not exist?*

- Generates X from pw.
- Update relay with X.

*Is Y and Bcrypt(k_B) available?*

- Use *v* to generate its session key *k_A*.
- Check that Bcrypt(k_B) comes from k_A. Abort here if it is incorrect.
- Encrypts data using *k_A*. 
- Connect to TCP ports of Relay.
- Update relay with *Bcrypt(k_A)*.

*Are ports stapled?*

- Send data over TCP


#### Recipient

*Initialize*

- Request to join

*Is X available?*

- Generate *v*, session key *k_B*, and hashed session key *H(k_B)* using PAKE from secret *pw*.
- Send the Relay *Bcrypt(k_B)*

*Is Bcrypt(k_A) available?*

- Verify that *Bcrypt(k_A)* comes from k_B
- Connect to TCP ports of Relay and listen.
- Once file is received, Send close signal to Relay.


#### Relay

*Is there a listener for sender and recipient?*

- Staple connections.
- Send out to all parties that connections are stapled.



# License

MIT

# Acknowledgements

Thanks...

- ...[@warner](https://github.com/warner) for the [idea](https://github.com/warner/magic-wormhole).
- ...[@tscholl2](https://github.com/tscholl2) for the [encryption gists](https://gist.github.com/tscholl2/dc7dc15dc132ea70a98e8542fefffa28).
- ...[@skorokithakis](https://github.com/skorokithakis) for [code on proxying two connections](https://www.stavros.io/posts/proxying-two-connections-go/).
- ...for making pull requests [@Girbons](https://github.com/Girbons), [@techtide](https://github.com/techtide), [@heymatthew](https://github.com/heymatthew), [@Lunsford94](https://github.com/Lunsford94), [@lummie](https://github.com/lummie), [@jesuiscamille](https://github.com/jesuiscamille), [@threefjord](https://github.com/threefjord), [@marcossegovia](https://github.com/marcossegovia), [@csleong98](https://github.com/csleong98), [@afotescu](https://github.com/afotescu), [@callmefever](https://github.com/callmefever), [@El-JojA](https://github.com/El-JojA), [@anatolyyyyyy](https://github.com/anatolyyyyyy), [@goggle](https://github.com/goggle), [@smileboywtu](https://github.com/smileboywtu)!