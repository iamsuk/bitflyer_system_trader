package controllers

import (
	"github.com/iamsuk/bitflyer_system_trader/app/models"
	"github.com/iamsuk/bitflyer_system_trader/bitfyler"
	"github.com/iamsuk/bitflyer_system_trader/config"
)


func StreamIngestionData() {
	var tickerChannel = make(chan bitfyler.Ticker)
	apiClient := bitfyler.New(config.Config.ApiKey, config.Config.APiSecret)
	//ticker(その瞬間の値段)を取り続け、tickerChannelにわたす
	go apiClient.GetRealTimeTicker(config.Config.ProductCode, tickerChannel)
	go func() {
		for ticker := range tickerChannel {
			for _, duration := range config.Config.Durations {
				isCreated := models.CreateCandleWithDuration(ticker, ticker.ProductCode, duration)
				if isCreated && duration == config.Config.TradeDuration {
					//todu
				}
			}
		}
	}()
}