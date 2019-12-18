package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gitlab.com/arcanecrypto/lnassets/server/larpc"
	"math"

	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcutil"
	"github.com/google/uuid"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/lnassets/server/bitmex"
)

var _ larpc.AssetServerServer = &AssetServer{}

type AssetServer struct {
	lncli          lnrpc.LightningClient
	db             *bolt.DB
	insecure       bool
	contracts      *bolt.Bucket
	port           int
	percentMargin  float64
	priceServerURL string
	bitmexApi      *bitmex.Bitmex

	// channels
	paymentsCh          chan larpc.Payment
	contractCh          chan larpc.ServerContract
	invoiceSubscription lnrpc.Lightning_SubscribeInvoicesClient
}

func (a AssetServer) NewContract(ctx context.Context, req *larpc.ServerNewContractRequest) (*larpc.ServerNewContractResponse, error) {
	log.Infoln("new contract request")

	// validate request is OK
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount can not be 0")
	}
	ok := assetIsSupported(req.Asset)
	if !ok {
		var supported []string
		for currency := range prices {
			supported = append(supported, currency)
		}
		return nil, fmt.Errorf("asset %s not supported, try one of: %+v", req.Asset, supported)
	}

	contract := larpc.ServerContract{
		Uuid:         uuid.New().String(),
		Asset:        req.Asset,
		Amount:       req.Amount,
		AmountSats:   convertPercentOfAssetToSats(req.Amount, req.Asset, 100),
		ClientHost:   req.Host,
		ContractType: req.ContractType,
	}

	// all contract types has a margin invoice
	marginInvoice, err := a.AddInvoice(contract.Uuid, lnrpc.Invoice{
		Value: int64(math.Round(float64(contract.AmountSats) * a.percentMargin / 100)),
		Memo:  contract.Uuid,
	})
	if err != nil {
		return nil, err
	}
	contract.MarginPayReq = marginInvoice.PaymentRequest

	switch req.ContractType {
	case larpc.ContractType_FUNDED:
		// create initiating invoice
		initiatingInvoice, err := a.AddInvoice(contract.Uuid, lnrpc.Invoice{
			Value: contract.AmountSats,
			Memo:  contract.Uuid,
		})
		if err != nil {
			return nil, err
		}
		contract.InitiatingPayReq = initiatingInvoice.PaymentRequest

	case larpc.ContractType_UNFUNDED:
		// nothing needs to be done

	default:
		return &larpc.ServerNewContractResponse{}, errors.New("contract type specified is not supported")
	}

	err = saveContract(a.db, a.contractCh, contract)
	if err != nil {
		return nil, fmt.Errorf("could not save contract: %w", err)
	}

	return &larpc.ServerNewContractResponse{
		Uuid:         contract.Uuid,
		MarginPayReq: contract.MarginPayReq,
		// If the contract is UNFUNDED type, this is just going to be an empty string
		// which is the default value anyways
		InitiatingPayReq: contract.InitiatingPayReq,

		PercentMargin: a.percentMargin,
		AssetPrice:    prices[req.Asset],
	}, nil
}

func assetIsSupported(asset string) bool {
	for currency := range prices {
		if currency == asset {
			return true
		}
	}
	return false
}

// convertPercentOfAssetToSats converts a percentage of an amount of a given asset to satoshis
func convertPercentOfAssetToSats(amount float64, asset string, percent float64) int64 {
	price := prices[asset]
	amountSat := (amount / price) * btcutil.SatoshiPerBitcoin
	return int64(math.Round(amountSat * percent / 100))
}

func saveContract(db *bolt.DB, contractCh chan larpc.ServerContract, contract larpc.ServerContract) error {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(contractsBucket)

		reqBytes, err := json.Marshal(contract)
		if err != nil {
			return err
		}

		return b.Put([]byte(contract.Uuid), reqBytes)
	})
	if err != nil {
		return err
	}

	// pass the saved contract on to the contractCh, in case someone is subscribed
	select {
	case contractCh <- contract:
	default:
	}

	return nil
}

func (a AssetServer) CloseContract(ctx context.Context, req *larpc.ServerCloseContractRequest) (*larpc.ServerCloseContractResponse, error) {
	log.Infof("received close contract request")

	if req.Uuid == "" {
		return nil, fmt.Errorf("uuid can not be empty")
	}

	var contract larpc.ServerContract
	err := a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(contractsBucket)

		rawContract := b.Get([]byte(req.Uuid))

		return json.Unmarshal(rawContract, &contract)
	})
	if err != nil {
		return nil, err
	}

	// close position on equal size bitmex
	_, _, err = a.bitmexApi.MarketSell(contract.Amount)
	if err != nil {
		return nil, fmt.Errorf("could not market sell: %w", err)
	}

	err = deleteContract(a.db, req.Uuid)
	if err != nil {
		return nil, err
	}

	return &larpc.ServerCloseContractResponse{}, nil
}

func (a AssetServer) ListAssets(ctx context.Context, req *larpc.ServerListAssetsRequest) (*larpc.ServerListAssetsResponse, error) {

	supportedAssets := make([]string, len(prices))

	for asset, _ := range prices {
		supportedAssets = append(supportedAssets, asset)
	}

	return &larpc.ServerListAssetsResponse{
		SupportedAssets:      supportedAssets,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}, nil
}

func deleteContract(db *bolt.DB, uuid string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(contractsBucket)

		return b.Delete([]byte(uuid))
	})
}

func (a AssetServer) SetPrice(asset string, amount float64) error {
	log.Infof("received set price request")

	_, ok := prices[asset]
	if !ok {
		return fmt.Errorf("asset does not exist: %s", asset)
	}

	prices[asset] = amount

	err := a.rebalanceContracts()
	if err != nil {
		return err
	}

	return nil
}

func savePayment(db *bolt.DB, paymentCh chan larpc.Payment, payment larpc.Payment) error {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(paymentsBucket)

		asByte, err := json.Marshal(payment)
		if err != nil {
			return err
		}

		uid := uuid.New()

		return b.Put([]byte(uid.String()), asByte)
	})
	if err != nil {
		log.Infof("could not save payment: %+v", payment)
		return err
	}

	select {
	case paymentCh <- payment: // put payment into channel
	default:
	}

	return nil
}

// calculateRebalanceAmount calculates the amount needed to rebalance a channel
func calculateRebalanceAmount(contract larpc.ServerContract) (rebalanceType, int64) {
	price := prices[contract.Asset]
	if price == 0 {
		return "", 0
	}

	expectedAmountSats := int64(math.Round(contract.Amount / price * btcutil.SatoshiPerBitcoin))

	logger := log.WithFields(logrus.Fields{
		"price":              price,
		"expectedAmountSats": expectedAmountSats,
		"currentAmountSats":  contract.AmountSats,
	})

	if contract.AmountSats > expectedAmountSats {
		// we have too many sats, and need to send some
		logger.Info("need to send sats")

		return SEND, contract.AmountSats - expectedAmountSats
	} else {
		// we have to few sats, and need to receive some
		logger.Info("need to receive sats")

		return RECEIVE, expectedAmountSats - contract.AmountSats
	}
}
