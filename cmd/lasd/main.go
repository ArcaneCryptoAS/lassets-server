package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/protobuf/ptypes/timestamp"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/ArcaneCryptoAS/lassets-server/bitmex"
	"github.com/ArcaneCryptoAS/lassets-server/build"
	"github.com/ArcaneCryptoAS/lassets-server/larpc"
	"github.com/ArcaneCryptoAS/lndutil"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	ErrContractNotOpen = errors.New("contract not open yet")
)

var (
	contractsBucket = []byte("contracts")
	paymentsBucket  = []byte("payments")
	defaultDBName   = "laserver.db"
)

var prices = map[string]float64{
	"USD": 0.00,
	"NOK": 0.00,
}

var (
	defaultLadPort       = 10455
	defaultRestPort      = 8080
	defaultLadDir        = cleanAndExpandPath("~/.las")
	defaultNetwork       = "regtest"
	defaultPercentMargin = 1.0

	// this should be changed to lnd-path when we start deploying it to servers
	defaultLndDir     = cleanAndExpandPath("~/.lnd")
	defaultLndRpcPort = "localhost:10009"
)

// define possible flag names here
const (
	flag_port               = "port"
	flag_rest_port          = "restport"
	flag_laddir             = "laddir"
	flag_network            = "network"
	flag_rebalancefrequency = "rebalancefrequency"
	flag_lnddir             = "lnddir"
	flag_lndrpchost         = "lndrpchost"
	flag_percentmargin      = "percentmargin"
	flag_insecure           = "insecure"
	flag_breakafter         = "breakafter"

	flag_bitmexapikey    = "bitmexapikey"
	flag_bitmexsecretkey = "bitmexsecretkey"
)

var log = logrus.New()

func main() {
	app := cli.NewApp()
	app.Name = "lasd"
	app.Version = build.Version()
	app.Usage = "Lightning Assets Server Daemon (lasd)"
	app.Flags = []cli.Flag{
		// lightning asset daemon flags
		cli.IntFlag{
			Name:  flag_port,
			Value: defaultLadPort,
			Usage: "port to run lightning asset grpc daemon on",
		},
		cli.IntFlag{
			Name:  flag_rest_port,
			Value: defaultRestPort,
			Usage: "port to run lightning asset rest server",
		},
		cli.StringFlag{
			Name:  flag_laddir,
			Usage: "the location of lad dir",
			Value: defaultLadDir,
		},
		cli.StringFlag{
			Name:  flag_network,
			Usage: "which bitcoin network to run on, regtest | testnet | mainnet",
			Value: defaultNetwork,
		},
		cli.Float64Flag{
			Name:  flag_percentmargin,
			Usage: "how many percent margin is necessary in a channel",
			Value: defaultPercentMargin,
		},
		cli.IntFlag{
			Name:  flag_rebalancefrequency,
			Usage: "how often to rebalance channels",
		},

		// flags specific to connecting to lnd
		cli.StringFlag{
			Name:  flag_lnddir,
			Usage: "the full path to lnd directory",
			Value: defaultLndDir,
		},
		cli.StringFlag{
			Name:  flag_lndrpchost,
			Usage: "host:port of lnd daemon",
			Value: defaultLndRpcPort,
		},
		cli.BoolFlag{
			Name:  flag_insecure,
			Usage: "whether the connection to the clients should be insecure or not",
		},

		cli.StringFlag{
			Name:   flag_bitmexapikey,
			Usage:  "api key for bitmex user. This should not be passed as cli flag, but as an environment variable ",
			EnvVar: "BITMEX_API_KEY",
		},
		cli.StringFlag{
			Name:   flag_bitmexsecretkey,
			Usage:  "secret key for bitmex user. This should not be passed as cli flag, but as an environment variable",
			EnvVar: "BITMEX_SECRET_KEY",
		},
	}
	app.Action = runLightningAssetDaemon

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("[lad]: %v", err)
	}
}

func runLightningAssetDaemon(c *cli.Context) error {
	// open and set up database
	ladDir := c.String(flag_laddir)
	if _, err := os.Stat(ladDir); os.IsNotExist(err) {
		os.Mkdir(ladDir, os.ModePerm) // 0777 permission
	}

	db, err := bolt.Open(path.Join(ladDir, defaultDBName), 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer db.Close()

	err = createBucketsIfNotExist(db)
	if err != nil {
		return err
	}

	// connect to lnd
	lncli, err := lndutil.NewLNDClient(lndutil.LightningConfig{
		LndDir:    c.String(flag_lnddir),
		Network:   c.String(flag_network),
		RPCServer: c.String(flag_lndrpchost),
	})
	if err != nil {
		return fmt.Errorf("could not connect to lnd: %w", err)
	}

	// create connection for daemon to listen on
	port := c.Int(flag_port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("could not listen: %w", err)
	}

	bitmexApi := bitmex.New(c.String(flag_bitmexapikey), c.String(flag_bitmexsecretkey))

	// create channel that new contracts and new payments are sent to
	contractCh := make(chan larpc.ServerContract)
	paymentCh := make(chan larpc.Payment)

	assetServer := AssetServer{
		lncli:              lncli,
		db:                 db,
		insecure:           c.Bool(flag_insecure),
		port:               c.Int(flag_port),
		percentMargin:      c.Float64(flag_percentmargin),
		bitmexApi:          bitmexApi,
		breakContractAfter: c.Int64(flag_breakafter),

		contractCh: contractCh,
		paymentsCh: paymentCh,
	}

	// create channel that listens to lnd invoices
	invoiceSubscription, err := lncli.SubscribeInvoices(context.Background(), &lnrpc.InvoiceSubscription{})
	if err != nil {
		return fmt.Errorf("could not subscribe to lnd invoices: %w", err)
	}
	go func() {
		err = assetServer.handleInvoices(db, invoiceSubscription)
		if err != nil {
			log.Fatalf("could not handle invoices: %w", err)
		}
	}()

	getPrice := func(asset string) float64 {
		return prices[asset]
	}
	// this go func connects to a bitmex websocket and updates
	// our saved price for any changes > 1 dollar compared to our saved price.
	// As a result of calling SetPrice it also rebalances all contracts on a price change
	go func() {
		err = bitmex.ListenToPrice(getPrice, assetServer.SetPrice)
		if err != nil {
			log.Fatalf("could not listen to bitmex price: %w", err)
		}
	}()

	// create grpc server that listens to grpc requests
	grpcServer := grpc.NewServer()
	larpc.RegisterAssetServerServer(grpcServer, assetServer)

	// start webserver that uses normal http / http2, used for communicating with front-end
	wrappedGrpc := grpcweb.WrapServer(grpcServer)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrappedGrpc.ServeHTTP(w, r)
	})

	router := mux.NewRouter()
	router.Use(headerMiddleware)
	router.PathPrefix("/").Handler(handler)

	go func() {
		log.Infoln("rest server listening on port", c.Int(flag_rest_port))
		res := http.ListenAndServe(fmt.Sprintf(":%d", c.Int(flag_rest_port)), router)
		log.Fatal(res)
	}()

	// if set, spawn a goroutine that rebalances all contracts every `--rebalancefrequency` seconds
	if c.Int(flag_rebalancefrequency) != 0 {
		go assetServer.rebalanceEvery(time.Second * time.Duration(c.Int(flag_rebalancefrequency)))
	}

	log.Infof("grpc server listening on port %d", port)
	err = grpcServer.Serve(lis)
	if err != nil {
		return fmt.Errorf("could not serve: %w", err)
	}

	return nil
}

// handleInvoices handles all incoming invoices for our client
// It only cares about settled invoices

// NOTE: MUST be run in a gorountine
func (a AssetServer) handleInvoices(db *bolt.DB, subscription lnrpc.Lightning_SubscribeInvoicesClient) error {

	for {
		inv, err := subscription.Recv()
		if err != nil {
			return fmt.Errorf("could not receive invoice from subscription: %w", err)
		}

		logger := log.WithFields(logrus.Fields{
			"paymentRequest": inv.PaymentRequest[0:10],
			"memo":           inv.Memo,
		})

		switch inv.State {
		case lnrpc.Invoice_OPEN:
			logger.Info("created new invoice")

		case lnrpc.Invoice_SETTLED:

			if inv.Memo == "" {
				// if memo is not registered. we don't need to deal with the payment,
				// it's something else running on this node
				continue
			}

			logger.Info("received payment")

			// lookup the contract using the uuid from the payment requests memo
			contractUUID := inv.Memo
			err = db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(contractsBucket)

				// get the contract from the DB
				var contract larpc.ServerContract
				asByte := b.Get([]byte(contractUUID))
				err := json.Unmarshal(asByte, &contract)
				if err != nil {
					return fmt.Errorf("could not unmarshal contract: %w", err)
				}
				logger := logger.WithField("uuid", contract.Uuid)

				// First, we update the contract based on the settled paymentrequest
				if inv.PaymentRequest == contract.MarginPayReq {
					contract.MarginPaid = true
					logger.Info("margin paid")
				} else if inv.PaymentRequest == contract.InitiatingPayReq {
					contract.InitiatingPaid = true
					logger.Info("initiating paid")
				} else {
					// if its neither a marginpayreq or initiatingpayreq, we assume
					// it is used for rebalancing a contract, and we reset the timer
					// to indicate that this contract is recently rebalanced and does
					// not need to be closed
					now := time.Now()
					contract.LastRebalancedAt = &timestamp.Timestamp{
						Seconds: int64(now.Second()),
						Nanos:   int32(now.Nanosecond()),
					}

					// save the contract with the latest RebalanceAt timestamp
					// nothing more needs to be done after this, therefore we return
					asByte, err = json.Marshal(contract)
					if err != nil {
						return fmt.Errorf("could not marshal contract: %w", err)
					}

					return b.Put([]byte(contract.Uuid), asByte)
				}

				// based on the contract type, we require either both margin and
				// initiating paymentrequests to be paid, or just the margin
				// if the contract
				switch contract.ContractType {

				case larpc.ContractType_FUNDED:
					if contract.InitiatingPaid && contract.MarginPaid {
						// contract is now open. To lock the price for the client, hedge position on bitmex

						// always convert to USD
						buyAmount := convertAssetAmount(contract.Asset, contract.Amount, "USD")
						resp, orderID, err := a.bitmexApi.MarketBuy(buyAmount)
						if err != nil {
							return fmt.Errorf("could not market buy: %w", err)
						}
						logger.WithFields(logrus.Fields{
							"status":  resp.Status,
							"orderID": orderID,
						}).Info("opened position on bitmex for funded contract")
					}

				case larpc.ContractType_UNFUNDED:
					if contract.MarginPaid {
						buyAmount := convertAssetAmount(contract.Asset, contract.Amount, "USD")
						resp, orderID, err := a.bitmexApi.MarketBuy(buyAmount)
						if err != nil {
							return fmt.Errorf("could not market buy: %w", err)
						}
						logger.WithFields(logrus.Fields{
							"status":  resp.Status,
							"orderID": orderID,
						}).Info("opened position on bitmex for unfunded contract")
					}

				default:
					log.Info("received unexpected contract type %d", contract.ContractType)
				}

				asByte, err = json.Marshal(contract)
				if err != nil {
					return fmt.Errorf("could not marshal contract: %w", err)
				}

				// save the contract with the new open status
				return b.Put([]byte(contract.Uuid), asByte)
			})
			if err != nil {
				logger.WithError(err).Error("could not update contract")
			} else {
				logger.Trace("successfully updated contract")
			}

		default:
			logger.Tracef("not handling invoice with state %s")
		}
	}
}

func headerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Headers", "x-grpc-web")
		w.Header().Add("Access-Control-Allow-Headers", "content-type")
		next.ServeHTTP(w, r)
	})
}

func createBucketsIfNotExist(db *bolt.DB) error {
	// create bucket if it doesnt exist
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(contractsBucket)
		if err != nil {
			return fmt.Errorf("could not create bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists(paymentsBucket)
		if err != nil {
			return fmt.Errorf("could not create bucket: %w", err)
		}
		// add additional buckets here
		return nil
	})
}

// reabalanceEvery rebalances all the contracts every Duration

// NOTE: Must be run in a goroutine
func (a AssetServer) rebalanceEvery(frequency time.Duration) {
	for {
		err := a.rebalanceContracts()
		if err != nil {
			log.WithError(err).Info("could not rebalance contracts in goroutine")
		}

		time.Sleep(frequency)
	}
}

type rebalanceType string

const SEND rebalanceType = "SEND"
const RECEIVE rebalanceType = "RECEIVE"

func (a AssetServer) rebalanceContracts() error {

	var contracts []larpc.ServerContract

	// extract all contracts from database
	err := a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(contractsBucket)

		// rebalance each contract
		return b.ForEach(func(k, v []byte) error {
			var contract larpc.ServerContract

			if err := json.Unmarshal(v, &contract); err != nil {
				return fmt.Errorf("could not unmarshal contract %q: %w", string(v), err)
			}

			contracts = append(contracts, contract)

			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("could not extract contracts from db: %w", err)
	}

	// rebalance all contracts
	for _, contract := range contracts {
		err = a.rebalanceContract(contract)
		if err != nil {
			if !errors.Is(err, ErrContractNotOpen) {
			log.WithError(err).WithField("uuid", contract.Uuid).
				Error("could not rebalance contract")
			}
		}
	}

	return nil
}

func (a AssetServer) rebalanceContract(contract larpc.ServerContract) error {
	if !contract.MarginPaid {
		return fmt.Errorf("margin not paid: %w", ErrContractNotOpen)
	}

	if contract.ContractType == larpc.ContractType_FUNDED && !contract.InitiatingPaid {
		return fmt.Errorf("initiating not paid: %w", ErrContractNotOpen)
	}

	lastRebalancedAt := time.Unix(contract.LastRebalancedAt.Seconds, int64(contract.LastRebalancedAt.Nanos))
	// if time of last rebalance is greater than 30 seconds, we close the contract
	if time.Now().Add(time.Second * 30).After(lastRebalancedAt) {
		_, err := a.CloseContract(context.Background(), &larpc.ServerCloseContractRequest{
			Uuid: contract.Uuid,
		})
		if err != nil {
			return fmt.Errorf("could not close inactive contract")
		}
		return nil
	}

	direction, rebalanceAmountSat := calculateRebalanceAmount(contract)

	if rebalanceAmountSat == 0 {
		return nil
	}

	client, cleanup, err := connectToLaClient(contract.ClientHost,
		a.insecure, "")
	if err != nil {
		return fmt.Errorf("could not connect to client: %w", err)
	}
	defer cleanup()

	if direction == SEND {
		// we need to send sats
		res, err := client.RequestPaymentRequest(context.Background(), &larpc.ClientRequestPaymentRequestRequest{
			AmountSat: rebalanceAmountSat,
		})
		if err != nil {
			return fmt.Errorf("could not request payment request: %w", err)
		}

		// TODO: Check amount is correct
		err = a.PayInvoice(contract.Uuid, res.PayReq)
		if err != nil {
			return fmt.Errorf("could not pay invoice: %w", err)
		}

		contract.AmountSats -= rebalanceAmountSat
	} else {
		// we need to request sats
		inv, err := a.AddInvoice(contract.Uuid, lnrpc.Invoice{
			Value: rebalanceAmountSat,
		})
		if err != nil {
			return fmt.Errorf("could not add invoice: %w", err)
		}

		_, err = client.RequestPayment(context.Background(), &larpc.ClientRequestPaymentRequest{
			PayReq: inv.PaymentRequest,
		})
		if err != nil {
			return fmt.Errorf("could not request payment: %w", err)
		}

		contract.AmountSats += rebalanceAmountSat
	}

	// save contract
	contract.NumUpdates++
	err = saveContract(a.db, a.contractCh, contract)
	if err != nil {
		return fmt.Errorf("could not save contract: %w", err)
	}

	return nil
}

type LadClient interface {
	// RequestPaymentRequest is used to close a contract with a specific uuid
	RequestPaymentRequest(ctx context.Context, in *larpc.ClientRequestPaymentRequestRequest, opts ...grpc.CallOption) (*larpc.ClientRequestPaymentRequestResponse, error)

	// RequestPayment is used to close a contract with a specific uuid
	RequestPayment(ctx context.Context, in *larpc.ClientRequestPaymentRequest, opts ...grpc.CallOption) (*larpc.ClientRequestPaymentResponse, error)
}

// connnectToLaClient opens a connection to a las
func connectToLaClient(address string, insecure bool,
	tlsPath string) (LadClient, func(), error) {

	// Create a dial options array.
	opts := []grpc.DialOption{}

	// There are three options to connect to a swap server, either insecure,
	// using a self-signed certificate or with a certificate signed by a
	// public CA.
	switch {
	case insecure:
		opts = append(opts, grpc.WithInsecure())

	case tlsPath != "":
		// Load the specified TLS certificate and build
		// transport credentials
		creds, err := credentials.NewClientTLSFromFile(tlsPath, "")
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))

	default:
		creds := credentials.NewTLS(&tls.Config{})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	clientConn, err := grpc.Dial(address, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to RPC server: %v",
			err)
	}

	client := larpc.NewAssetClientClient(clientConn)

	cleanUp := func() {
		clientConn.Close()
	}

	return client, cleanUp, nil
}
