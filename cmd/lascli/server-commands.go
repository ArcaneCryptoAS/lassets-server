package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/ArcaneCryptoAS/lassets-server/larpc"
)

var log = logrus.New()

var closeContractCommand = cli.Command{
	Name:     "closecontract",
	Category: "Contracts",
	Usage:    "Close a contract",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "uuid",
			Usage: "the uuid of the contract",
		},
	},
	Action: closeContract,
}

func closeContract(ctx *cli.Context) error {
	conn, cleanup := connectToServerDaemon(ctx.GlobalInt(flag_rpcport))
	defer cleanup()

	uuid := ctx.String("uuid")

	_, err := conn.CloseContract(context.Background(), &larpc.ServerCloseContractRequest{
		Uuid: uuid,
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"UUID": uuid,
		}).WithError(err).Error("could not close contract")
		return err
	}

	log.WithField("uuid", uuid).Infof("closed contract")

	return nil
}

