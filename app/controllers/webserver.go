package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/iamsuk/bitflyer_system_trader/app/models"
	"github.com/iamsuk/bitflyer_system_trader/config"
)


var templates = template.Must(template.ParseFiles("app/views/google.html.j2"))

func viewChartHandler(w http.ResponseWriter, r *http.Request) {
	limit := 100 
	//変更
	durationTime := config.Config.TradeDuration
	//GEtAllCandleでlimit文のDataFrameをとってっくる
	df, _ := models.GetAllCandle(config.Config.ProductCode, durationTime, limit)

	err := templates.ExecuteTemplate(w, "google.html.j2", df.Candles)
	if err!=nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type JSONError struct {
	Error	string	`json:"error"`
	Code 	int 	`json:"code"`
}

func APIError(w http.ResponseWriter, errMessage string, code int) {
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(code)
	jsonError, err := json.Marshal(JSONError{Error: errMessage, Code: code})
	if err != nil {
		log.Fatal(err)
	}
	_, _ = w.Write(jsonError)
}

var apiValidPath = regexp.MustCompile("^/api/candle/$")

func apiMakeHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := apiValidPath.FindStringSubmatch(r.URL.Path)
		if len(m) == 0 {
			APIError(w, "Not found", http.StatusNotFound)
		}
		fn(w,r)
	}
}

func apiCandleHandler(w http.ResponseWriter, r *http.Request) {
	productCode := r.URL.Query().Get("product_code")
	if productCode =="" {
		APIError(w, "no pruduct_code", http.StatusNotFound)
	}
	strLimit := r.URL.Query().Get("limit")
	limit, err := strconv.Atoi(strLimit)
	if err!=nil || strLimit=="" || limit < 0 || limit > 1000 {
		limit = 1000
	}
	duration := r.URL.Query().Get("duration")
	if duration == "" {
		duration = "1m"
	}
	durationTime := config.Config.Durations[duration]

	df, err := models.GetAllCandle(productCode, durationTime, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}

	js, err := json.Marshal(df)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(js)
}

func StartWebServer() error {
	http.HandleFunc("/api/candle/", apiMakeHandler(apiCandleHandler))
	http.HandleFunc("/chart/", viewChartHandler)
	return http.ListenAndServe(fmt.Sprintf(":%d", config.Config.Port), nil)
}