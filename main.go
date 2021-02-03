package main

import (
	"github.com/iamsuk/bitflyer_system_trader/app/controllers"
	"github.com/iamsuk/bitflyer_system_trader/config"
	"github.com/iamsuk/bitflyer_system_trader/utils"
)

func main() {
	utils.LoggingSettings(config.Config.LogFile)
	controllers.StreamIngestionData()
	controllers.StartWebServer()
}
