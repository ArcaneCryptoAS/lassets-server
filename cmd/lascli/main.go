package main

import (
	"fmt"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"os"

	"github.com/ArcaneCryptoAS/lassets-server/build"
	"github.com/ArcaneCryptoAS/lassets-server/larpc"
)

// default value for flags
const (
	defaultRPCPort = 10455
)

// all flags for this package
const (
	flag_rpcport = "rpcport"
)

func main() {
	app := cli.NewApp()
	app.Name = "lascli"
	app.Version = build.Version()
	app.Usage = "control plane for your Lightning Assets Server"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  flag_rpcport,
			Value: defaultRPCPort,
			Usage: "port of la daemon",
		},
	}
	app.Commands = []cli.Command{
		closeContractCommand,
	}

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}

// connectToServerDaemon opens a connection to the server daemon
func connectToServerDaemon(rpcPort int) (larpc.AssetServerClient, func()) {
	// Load the specified TLS certificate and build transport credentials
	// with it.
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	rpcServer := fmt.Sprintf("localhost:%d", rpcPort)

	conn, err := grpc.Dial(rpcServer, opts...)
	if err != nil {
		log.Fatalf("unable to connect to RPC server: %w", err)
	}

	cleanUp := func() {
		conn.Close()
	}

	return larpc.NewAssetServerClient(conn), cleanUp
}
