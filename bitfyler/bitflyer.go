package bitfyler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)


const baseURL = "https://api.bitflyer.com/v1/"

type APIClient struct {
	key        string
	secret     string
	httpClient *http.Client
}

func New(key,secret string) *APIClient {
	apiClient := &APIClient{key,secret, &http.Client{}}
	return apiClient
}

func (api APIClient) header(method, endpoint string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(),10)
	log.Println(timestamp)
	message := timestamp + method + endpoint + string(body)

	
	mac := hmac.New(sha256.New,[]byte(api.secret))
	//put message to mac
	_, err := mac.Write([]byte(message))
	if err!=nil {
		log.Fatal(err)
	}
	sign := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"ACCESS-KEY":api.key,
		"ACCESS-TIMESTAMP":timestamp,
		"ACCESS-SIGN":sign,
		"Content-Type":"application/json",
	}
}

func(api *APIClient) doRequest(method,urlPath string,query map[string]string,data []byte) (body []byte ,err error) {
	//check if baseURL,apiURL exists
	baseURL,err := url.Parse(baseURL)
	if err != nil {
		log.Fatalln(err)
		return nil,err 
	}
	apiURL, err := url.Parse(urlPath)
	if err != nil {
		log.Fatalln(err)
		return nil,err 
	}
	//concatenate baseURL and apiURL
	endpoint := baseURL.ResolveReference(apiURL).String()
	//create *Response struct
	//*Response has URL struc (which is in net/url)
	req,err := http.NewRequest(method,endpoint,bytes.NewBuffer(data))
	if err != nil {
		log.Fatalln(err)
		return nil,err 
	}
	//Query parses RawQuery(like a=1&b=2) and convert to map(like map[key]value{"a":1,"b":2}) with high accessibility.
	q := req.URL.Query()
	for key, value := range query{
		q.Add(key,value)
	}
	//RawQuery can be set by encoding 
	req.URL.RawQuery = q.Encode()
	//add to header
	for key,value := range api.header(method,req.URL.RequestURI(),data) {
		req.Header.Add(key,value)
	}


	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil,err 
	}
	defer resp.Body.Close()
	body,err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil,err 
	}
	return body,nil 
}

type Balance struct {
	CurrentCode string `json:"currency_code"`
	Amount float64 `json:"amount"`
	Available float64 `json:"available"`
}

func (api *APIClient) GetBalance() ([]Balance, error) {
	url := "me/getbalance"
	resp,err := api.doRequest("GET",url,map[string]string{},nil)
	log.Printf("url=%s, resp=%s",url,string(resp))
	if err != nil {
		log.Printf("action=GetBalance line=103 err=%s",err.Error())
		return nil,err
	}

	var balance []Balance
	err = json.Unmarshal(resp,&balance)
	if err != nil {
		log.Printf("action=GetBalance line=111 err=%s",err.Error())
		return nil,err
	}
	return balance,nil
}



//Public 
	
type Ticker struct {
	ProductCode     string  `json:"product_code"`
	State           string  `json:"state"`
	Timestamp       string  `json:"timestamp"`
	TickID          int     `json:"tick_id"`
	BestBid         float64 `json:"best_bid"`
	BestAsk         float64 `json:"best_ask"`
	BestBidSize     float64 `json:"best_bid_size"`
	BestAskSize     float64 `json:"best_ask_size"`
	TotalBidDepth   float64 `json:"total_bid_depth"`
	TotalAskDepth   float64 `json:"total_ask_depth"`
	MarketBidSize   float64 `json:"market_bid_size"`
	MarketAskSize   float64 `json:"market_ask_size"`
	Ltp             float64 `json:"ltp"`
	Volume          float64 `json:"volume"`
	VolumeByProduct float64 `json:"volume_by_product"`
}

func (t *Ticker) GetMidPrice() float64 {
	return (t.BestBid + t.BestAsk)/2
}

func (t *Ticker) DateTime() time.Time {
	jstLocation := time.FixedZone("Asia/Tokyo", 9*60*60)
	datetime,err := time.Parse(time.RFC3339,t.Timestamp)
	if err != nil {
		log.Printf("action=DateTime err=%s",err.Error())
	}
	datetime = datetime.In(jstLocation)
	return datetime
}

func (t *Ticker) TruncateDateTime(duration time.Duration) time.Time {
	//12h:12m:12s => truncate(time.Hour) => 12h:0m:0s
	return t.DateTime().Truncate(duration)
}

//deprecated!
func (api *APIClient) GetTicker(productCode string) (*Ticker, error) {
	url := "ticker"
	resp, err := api.doRequest("GET",url,map[string]string{"product_code":productCode},nil)
	if err != nil {
		log.Printf("action=GetTicker line=114 err=%s",err.Error())
		return nil,err 
	}
	var ticker Ticker
	err = json.Unmarshal(resp,&ticker)
	if err != nil {
		return nil,err 
	}
	return &ticker,nil
}


//JSONRPC realtime fetcher 
//JSONRPC MUST include method,params,id
type JsonRPC2 struct {
	Version string 		`json:"jsonrpc"`
	Method 	string 		`json:"method"`
	Params 	interface{} `json:"params"`
	Result 	interface{} `json:"result,omitempty"`
	Id 		*int		`json:"id,omitempty"`
}

type SubscribeParams struct {
	Channel string `json:"channel"`
}

func (api *APIClient) GetRealTimeTicker(symbol string,ch chan<- Ticker) {
	u := url.URL{Scheme: "wss",Host:"ws.lightstream.bitflyer.com",Path: "/json-rpc"}
	log.Printf("connecting to %s",u.String())

	//c Conn type represents a websocket connection.
	c, _, err := websocket.DefaultDialer.Dial(u.String(),nil)
	if err != nil {
		log.Fatal("dial:",err)
		return 
	}
	defer c.Close()

	//channel name
	channel := fmt.Sprintf("lightning_ticker_%s",symbol)
	//WriteJSON(v interface{}) writes the JSON encoding of v as a message.
	if err := c.WriteJSON(&JsonRPC2{Version: "2.0", Method: "subscribe", Params: &SubscribeParams{channel}}); err != nil {
		log.Fatal("subscribe:", err)
		return
	}

	OUTER:
	//1. write JsonRPC to Conn, and send. AUTOMATICALLY receive return and save it to Conn
	//2. make empty *JsonRPC2 struct.
	//3. Reads Conn if return exists. if so, save it to message AS a STRUCT
	//4. in bitflyer api, client method is "channelMessage" 
	//5. get message.Params.message which is map. convert map to JSON marchaTic
	//6. save marshaTic as Ticker struct
	//7. send channel ticker (this func is executed with go func())
		for {
			message := new(JsonRPC2) //pointer
			//ReadJSON(v interface{}) reads the next JSON-encoded message from the connection(=c) and stores it in the value pointed to by v(=message).
			if err := c.ReadJSON(message); err != nil {
				log.Println("read:",err)
				return 
			}
			

			if message.Method == "channelMessage" {
				// log.Printf("message.Params=%+v",message.Params)


				//message.Params: {"dsfsd":sdf,"message":{"product_code":"BTC_JPY","state" ...}}

				switch v := message.Params.(type) {
				case map[string]interface{}:
					for key,binary := range v {
						if key == "message" {
							//change it to Json
							marshaTic, err := json.Marshal(binary)
							if err != nil {
								continue OUTER
							}
							var ticker Ticker
							if err := json.Unmarshal(marshaTic,&ticker); err!=nil {
								continue OUTER
							}
							ch <- ticker
						}
					}
				}
			}
		}
}