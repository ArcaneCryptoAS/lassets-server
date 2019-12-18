## Lightning Assets Server
This project is the server side of a naive implementation of [lightning
 assets](http://research.paradigm.xyz/RainbowNetwork.pdf).
 
The client side can be found here: [lightning assets client.](https://github.com/ArcaneCryptoAS/lassets-client) You need both a server and a client to set it up.

The purpose of this server is to create synthetic assets on the lightning
 network, by opening "contracts". The client and server will
  continuously rebalance the contract, thereby making sure the satoshi balance
   between them is always X [ASSET]. The asset of the contract
    can be any asset both the client and server support. To determine the
     price of the contract, the server and client have to agree on an oracle. As of today, this common price feed is [bitmex](https://bitmex.com).

Sometime during the new year(january 2020), we will host a server you can connect to using just the client.
 
### Installing  
First download the project
```
go get -u github.com/ArcaneCryptoAS/lassets-server
```
To install the project, run in the project root directory
```
go install ./...
```
This will install two new commands in your $GOBIN
```
lasd   # The daemon
lascli # Used to interface with the daemon
```

##### Run on regtest
1.. Bring up the necessary docker containers, one bitcoind node and two lnd-nodes:
```
$ docker-compose up
```

2.. Open a new terminal and connect the two nodes
```
$ scripts/connect-alice-bob.sh
```

3.. Start the daemon
```
$ ./lasd
```

Now you're ready to go! Install the [Lightning Assets Client](https://github.com/ArcaneCryptoAS/lassets-client), and get dirty!

##### Run on testnet
It should automatically connect to your testnet lnd node
```
lasd --network=testnet
```

### Contributions 
Contributions are very welcome, just go ahead and open issues/pull requests.

### Required dependencies
### lnd
The project requires a lnd node running on your machine, regtest, testnet and
 mainnet is supported. Check out the official repo for installation
  instructions: https://github.com/lightningnetwork/lnd

### Optional dependencies
#### grpc-gateway
If you want to make changes to the proto file, you need to be able to recompile the .proto files in the larpc folder. There is a script `./gen_protos.sh` to generate new ones.
Installation instructions copied from [official repo](https://github.com/grpc).

The grpc-gateway requires a local installation of the Google protocol buffers
 compiler protoc v3.0.0 or above. To check if you already have this installed
 , run `protoc --version`. If you do not, please install this via your local
  package manager or by downloading one of the releases from the official repository:
  
https://github.com/protocolbuffers/protobuf/releases

Then use go get -u to download the following packages:

```bash
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
go get -u github.com/golang/protobuf/protoc-gen-go
```
This will place three binaries in your $GOBIN;
```text
protoc-gen-grpc-gateway
protoc-gen-swagger
protoc-gen-go
```

Make sure that your $GOBIN is in your $PATH.

