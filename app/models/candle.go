package models

import (
	"fmt"
	"log"
	"time"

	"github.com/iamsuk/bitflyer_system_trader/bitfyler"
)


type Candle struct {
	ProductCode string			`json:"product_code"`
	Duration  	time.Duration	`json:"duration"`
	Time 		time.Time		`json:"time"`
	Open 		float64			`json:"open"`
	Close		float64			`json:"close"`
	High		float64			`json:"high"`
	Low 		float64			`json:"low"`
	Volume 		float64			`json:"volume"`
}

//Candle Structを編成
func NewCandle(productCode string, duration time.Duration, timeDate time.Time, open, close, high, low, volume float64) *Candle {
	return &Candle{
		productCode,
		duration,
		timeDate,
		open,
		close,
		high,
		low,
		volume,
	}
}

func (c *Candle) TableName() string {
	return GetCandleTableName(c.ProductCode, c.Duration)
}

//Candleをデータベースに書きこみ
func (c *Candle) Create() error {
	cmd := fmt.Sprintf("INSERT INTO %s (time, open, close, high, low, volume) VALUES (?, ?, ?, ?, ?, ?)", c.TableName())
	_, err := DbConnection.Exec(cmd, c.Time.Format(time.RFC3339), c.Open, c.Close, c.High, c.Low, c.Volume)
	if err!=nil {
		return err 
	}
	return nil
}

//例えば１分ごとにキャンドルを作成するとき、毎秒受信するキャンドルデータを既存の該当するキャンドルに上書きする
func (c *Candle) Save() error {
	cmd := fmt.Sprintf("UPDATE %s SET open = ?, close = ?, high = ?, low = ?, volume = ? WHERE time = ?", c.TableName())
	_, err := DbConnection.Exec(cmd, c.Open, c.Close, c.High, c.Low, c.Volume, c.Time.Format(time.RFC3339))
	if err!=nil {
		return err 
	}
	return nil
}

//productCode duration dateTimeをもとにCandleを取得しStructに代入
//durationに応じてデータベースを選択する
//データベース＝＞Struct
func GetCandle(productCode string, duration time.Duration, dateTime time.Time) *Candle {
	tableName := GetCandleTableName(productCode, duration)
	cmd := fmt.Sprintf("SELECT time, open, close, high, low, volume FROM %s WHERE time = ?", tableName)
	row := DbConnection.QueryRow(cmd, dateTime.Format(time.RFC3339))
	var candle Candle 
	err := row.Scan(&candle.Time, &candle.Open, &candle.Close, &candle.High, &candle.Low, &candle.Volume)
	if err!=nil {
		return nil 
	}
	return NewCandle(productCode, duration, candle.Time, candle.Open, candle.Close, candle.High, candle.Low, candle.Volume)
}

//Duration(ex: 1h) における新しい時間(ex 14:00)のときデータベースにrowを追加。
//その後はrowがすでにあるので更新
//rowを作成したときtrueを、以外はfalseを返す。
//GetRealTimeTickerでとったtickerを渡す
func CreateCandleWithDuration(ticker bitfyler.Ticker, productCode string, duration time.Duration) bool {
	//tickerからtimeをdurationでtruncateしながら取得、GetCandleでproductCodeデータベースにtime時のキャンドル情報があるか問い合わせ。
	currentCandle := GetCandle(productCode, duration, ticker.TruncateDateTime(duration))
	//tickerには毎取引時の値段がある　midpriceがその瞬間のとりひきの中間
	price := ticker.GetMidPrice()
	//たとえば一時間間隔のチャートで
	//新しい１時間に突入したしゅんかん : 14:00とする
	//GetCandleでproductCode/1hourデータベースから14時の情報を取りたいがnilが帰ってくる。
	//このときあらたに14時のROWを作成したい。
//もしcurrentCandleがなければ作る
	if currentCandle == nil {
		//14時rowのすべての初期値は14:00時のpriceにする。
		candle := NewCandle(productCode, duration, ticker.TruncateDateTime(duration), price, price, price, price, ticker.Volume)
		err := candle.Create()
		if err!=nil {
			log.Fatal(err)
		}
		return true
	}
//もしcurrentCandle（前の瞬間に記述された）があり、現瞬間のpriceがより高ければ更新する
	if currentCandle.High <= price {
		currentCandle.High = price
	} else if currentCandle.Low >= price {
		currentCandle.Low = price
	} 

	currentCandle.Volume += ticker.Volume
	//その時間の終値は、その時間の最後の瞬間の値段
	currentCandle.Close = price
	err := currentCandle.Save()
	if err!=nil {
		log.Fatal(err)
	}
	return false
}


//productCode.durationで指定したデータベースからCandleをlimitしつつ取る
func GetAllCandle(productCode string, duration time.Duration, limit int) (dfCandle *DataFrameCandle, err error) {
	tableName := GetCandleTableName(productCode, duration)
	cmd := fmt.Sprintf(`SELECT * FROM (
		SELECT time, open, close, high, low, volume FROM %s ORDER BY time DESC LIMIT ?) ORDER BY time ASC;`,tableName)
	rows, err := DbConnection.Query(cmd, limit)
	if err!=nil {
		return 
	}
	defer rows.Close()

	//dfCandle gonna contain []Candle
	dfCandle = &DataFrameCandle{}
	dfCandle.ProductCode = productCode
	dfCandle.Duration = duration
	for rows.Next() {
		var candle Candle
		candle.ProductCode = productCode
		candle.Duration = duration
		//なぜかデータベースから取得したtimeが+0900を２ド含む
		_ = rows.Scan(&candle.Time, &candle.Open, &candle.Close, &candle.High, &candle.Low, &candle.Volume)

		dfCandle.Candles = append(dfCandle.Candles, candle)
	}
	err = rows.Err()
	if err!=nil {
		return 
	}
	return dfCandle, nil
}