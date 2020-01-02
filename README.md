## Lightning Assets Server, non-custodial stable coins on lightning, using only bitcoin
This project is the server side of a naive implementation of [lightning
 assets](http://research.paradigm.xyz/RainbowNetwork.pdf).
 
The client can be found here: [lightning assets client.](https://github.com/ArcaneCryptoAS/lassets-client) You need both a server and a client to set it up.

#### What are lightning assets?
With this project, you can circumvent the problem of volatility in Bitcoin, and "peg" your lightning balance to
a stable(or unstable) currency of your choice. This can be your local currency, dollar, euro, ethereum etc.
It is non-custodial, meaning noone can take money from you, yay :tada:

#### How does it work
As a client, you open "contracts" with the server. The client and server will continuously rebalance the
contract, by sending sats client --> server if the price has dropped, or server --> client if the price
has increased. The goal is to ensure the balance of the contract is always X [ASSET]. The asset of the contract
can be any asset both the client and server support. To determine the price of the contract, the server and
client have to agree on an oracle. As of today, this common price feed is [bitmex](https://bitmex.com).

You do not need to run a server to test the project, only a client, which comes configured out of the box
to connect to a server we are running.
 
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

If you already have a lnd-node you want to connect to, run `lasd --network=testnet` and everything should connect.

### Run on regtest
##### Install direnv
```shell script
# Installs direnv, a manager for environment variables
sudo apt-get install direnv
# you need to hook direnv into your shell: https://direnv.net/docs/hook.html
cp .envrc-example .envrc
```

Now hook direnv into your shell. [Instructions found here](https://direnv.net/docs/hook.html).

##### Create a user on bitmex
Second, you need a user on bitmex to test the project properly.
1. Sign up on https://testnet.bitmex.com
2. Create an API key
3. copy-paste the `API_KEY` and `SECRET_KEY` into the .envrc file


Then set everything up:
```shell script
# Bring up the necessary docker containers, one bitcoind node and two lnd-nodes
docker-compose up -d 
scripts/connect-alice-bob.sh # Funds and connects the nodes

# Start the daemon
./lasd
```

Now you're ready to go! The only remaining step is to set up a [Lightning Assets Client](https://github.com/ArcaneCryptoAS/lassets-client), and get dirty!

### Contributions 
Contributions are very welcome, just go ahead and open issues/pull requests.

### Required dependencies
### lnd
The project requires a lnd node running on your machine, regtest, testnet and
 mainnet is supported. Check out the official repo for installation
  instructions: https://github.com/lightningnetwork/lnd
  
### Optional dependencies
### Docker
Instructions can be found here: https://docs.docker.com/install/linux/docker-ce/ubuntu/ 

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

