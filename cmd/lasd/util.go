package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ArcaneCryptoAS/lassets-server/larpc"
	"github.com/lightningnetwork/lnd/lnrpc"
)

// PayInvoice does not exist in grpc, but is a util method defined on an AssetServer
func (a AssetServer) PayInvoice(uuid, paymentRequest string) error {

	res, err := a.lncli.SendPaymentSync(context.Background(), &lnrpc.SendRequest{
		PaymentRequest: paymentRequest,
	})
	if err != nil {
		return err
	}

	if res.PaymentError != "" {
		return fmt.Errorf("could not send payment: %s", res.PaymentError)
	}

	invoice, err := a.lncli.DecodePayReq(context.Background(), &lnrpc.PayReqString{
		PayReq: paymentRequest,
	})
	if err != nil {
		return err
	}

	err = savePayment(a.db, a.paymentsCh, larpc.Payment{
		ContractUuid:   uuid,
		AmountSat:      invoice.NumSatoshis,
		PaymentRequest: paymentRequest,
		Outbound:       true,
	})
	if err != nil {
		return err
	}

	log.WithField("paymentRequest", paymentRequest).Info("paid")

	return nil
}

// AddInvoice does not exist in grpc, but is a util method defined on an AssetServer
func (a AssetServer) AddInvoice(uuid string, invoice lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error) {
	log.Info("adding invoice")

	res, err := a.lncli.AddInvoice(context.Background(), &invoice)
	if err != nil {
		return nil, err
	}

	err = savePayment(a.db, a.paymentsCh, larpc.Payment{
		ContractUuid:   uuid,
		AmountSat:      invoice.Value,
		PaymentRequest: res.PaymentRequest,
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	return filepath.Clean(os.ExpandEnv(path))
}
