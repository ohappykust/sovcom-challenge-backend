package cryptocurrency

import (
	"context"
	"currency-parser/internal/app/sending/amqp"
	"currency-parser/pkg/core/config"
	"currency-parser/pkg/core/database"
	logger "currency-parser/pkg/logging"
	LoggerTypes "currency-parser/pkg/logging/types"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"libs/contracts/currency"
	"time"
)

func Consumer(tickerFrom string, tickerTo string) {
	tickerFull := tickerFrom + tickerTo
	logger.Log(LoggerTypes.INFO, "[Currency-parser | Cryptocurrency | "+tickerFull+"] Start parsing.", nil)
	connectionString := fmt.Sprintf(CryptoCurrencyURL, tickerFull)
	c, _, err := websocket.DefaultDialer.Dial(connectionString, nil)
	if err != nil {
		logger.Log(LoggerTypes.ERROR, "[Currency-parser | Cryptocurrency | "+tickerFull+"] Could not connect to websocket", err)
	}
	database.Redis.SAdd(context.Background(), config.RedisTickersGroupsSet, tickerFull)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				logger.Log(LoggerTypes.ERROR, "[Currency-parser | Cryptocurrency | "+tickerFull+"] Error while reading message.", err)
				c, _, err = websocket.DefaultDialer.Dial(connectionString, nil)
				continue
			}

			cryptoCurrency := CryptocurrencyIncoming{}
			err = json.Unmarshal(message, &cryptoCurrency)
			if err != nil {
				logger.Log(LoggerTypes.ERROR, "[Currency-parser | Cryptocurrency | "+tickerFull+"] Error while unmarshaling incoming message.", err)
			}

			outJson, err := json.Marshal(cryptoCurrency)
			if err != nil {
				logger.Log(LoggerTypes.ERROR, "[Currency-parser | Cryptocurrency | "+tickerFull+"] Error while marshaling incoming message.", err)
				continue
			}
			database.Redis.HSet(context.Background(), TickersGroupName+":"+tickerFull, cryptoCurrency.EventTime, outJson)

			go amqp.SendCurrencyUpdateToCurrency(currency.CurrencyUpdateRequest{TickerGroup: tickerFull,
				TickerFrom: tickerFrom, TickerTo: tickerTo, Data: cryptoCurrency})
			go amqp.SendCurrencyUpdateToDeals(currency.CurrencyUpdateRequest{TickerGroup: tickerFull,
				TickerFrom: tickerFrom, TickerTo: tickerTo, Data: cryptoCurrency})
		}
	}()
	go func() {
		time.Sleep(time.Minute * 9)
		c.Close()
		c, _, err = websocket.DefaultDialer.Dial(connectionString, nil)
	}()
}
