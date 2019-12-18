package bitmex

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/qct/bitmex-go/swagger"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

type Bitmex struct {
	swaggerOrderApi *swagger.OrderApiService
	ctx             context.Context
}

var log = logrus.New()

// Create a new Bitmex api that can market buy/sell and limit buy/sell
func New(apiKey, secretKey string) *Bitmex {

	apiClient := swagger.NewAPIClient(swagger.NewConfiguration())
	auth := context.WithValue(context.TODO(), swagger.ContextAPIKey, swagger.APIKey{
		Key:    apiKey,
		Secret: secretKey,
	})

	apiClient.ChangeBasePath("https://testnet.bitmex.com/api/v1")

	return &Bitmex{swaggerOrderApi: apiClient.OrderApi, ctx: auth}
}

func (o *Bitmex) MarketBuy(orderQty float64) (resp *http.Response, orderId string, err error) {

	params := map[string]interface{}{
		"symbol":   "XBTUSD",
		"ordType":  "Market",
		"orderQty": float32(orderQty),
	}
	order, response, err := o.swaggerOrderApi.OrderNew(o.ctx, "XBTUSD", params)
	if err != nil || response.StatusCode != 200 {
		return response, order.OrderID, err
	}
	return response, order.OrderID, nil
}

func (o *Bitmex) MarketSell(orderQty float64) (resp *http.Response, orderId string, err error) {

	params := map[string]interface{}{
		"symbol":   "XBTUSD",
		"ordType":  "Market",
		"orderQty": float32(-orderQty),
	}
	order, response, err := o.swaggerOrderApi.OrderNew(o.ctx, "XBTUSD", params)
	if err != nil || response.StatusCode != 200 {
		return response, order.OrderID, err
	}
	return response, order.OrderID, nil
}

func (o *Bitmex) LimitBuy(orderQty float64, price float64) (resp *http.Response, orderId string, err error) {
	if price <= 0 {
		return nil, "", errors.New("price must be positive")
	}

	params := map[string]interface{}{
		"symbol":   "XBTUSD",
		"ordType":  "Limit",
		"orderQty": float32(orderQty),
		"price":    price,
	}
	order, response, err := o.swaggerOrderApi.OrderNew(o.ctx, "XBTUSD", params)
	if err != nil || response.StatusCode != 200 {
		return response, order.OrderID, err
	}
	return response, order.OrderID, nil
}

func (o *Bitmex) LimitSell(orderQty float64, price float64) (resp *http.Response, orderId string, err error) {

	if price <= 0 {
		return nil, "", errors.New("price must be positive")
	}

	params := map[string]interface{}{
		"symbol":   "XBTUSD",
		"ordType":  "Limit",
		"orderQty": float32(-orderQty),
		"price":    price,
	}
	order, response, err := o.swaggerOrderApi.OrderNew(o.ctx, "XBTUSD", params)
	if err != nil || response.StatusCode != 200 {
		return response, order.OrderID, err
	}
	return response, order.OrderID, nil
}

// ListenToPrice opens a websocket to bitmex, and extracts
// latest price updates

// NOTE: MUST be run in a goroutine
func ListenToPrice(getPrice func(asset string) float64,
	setPrice func(asset string, price float64) error) error {

	// we subscribe to XBTUSD instrument updates. This includes several different updates
	// but we only care about priceUpdates
	const wsURL = "wss://www.bitmex.com/realtime?subscribe=instrument:XBTUSD"
	conn, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		log.Fatalf("could not dial bitmex: %w", err)
	}

	type price struct {
		Symbol            string    `json:"symbol"`
		LastPrice         float64   `json:"lastPrice"`
		LastTickDirection string    `json:"lastTickDirection"`
		LastChangePcnt    float64   `json:"lastChangePcnt"`
		Timestamp         time.Time `json:"timestamp"`
	}

	type priceUpdate struct {
		Table  string  `json:"table"`
		Action string  `json:"action"`
		Data   []price `json:"data"`
	}

	for {
		var msg string
		err = websocket.Message.Receive(conn, &msg)
		if err != nil {
			log.WithError(err).Error("could not receive from bitmex websocket")
		}

		// TODO: Try to open new connection
		if err == io.EOF {
		}

		var lastPrice priceUpdate

		err := json.Unmarshal([]byte(msg), &lastPrice)
		if err != nil {
			log.WithError(err).WithField("msg", msg).Error("could not unmarshal message")
		}
		// the priceUpdate response from bitmex is not unique and many responses unmarshal successfully
		// we only care about the ones where the Data array has data
		if len(lastPrice.Data) == 0 {
			continue
		}
		if lastPrice.Data[0].LastPrice == 0 {
			continue
		}

		data := lastPrice.Data[0]

		log.WithFields(logrus.Fields{
			"symbol":            data.Symbol,
			"lastPrice":         data.LastPrice,
			"lastTickDirection": data.LastTickDirection,
			"lastChangePercent": data.LastChangePcnt,
		}).Trace("new price")

		if math.Abs(data.LastPrice-getPrice("USD")) > 1 {
			log.WithFields(logrus.Fields{
				"savedPrice": getPrice("USD"),
				"newPrice":   data.LastPrice,
			}).Info("new price diff is high, saving new price")

			// update the price
			err = setPrice("USD", data.LastPrice)
			if err != nil {
				log.WithError(err).Error("could not set new bitmex price")
			}
		}
	}
}
