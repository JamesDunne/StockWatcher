package stocks

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestJSONMarshal(t *testing.T) {
	v := StockDetail{
		Stock: Stock{
			StockID:          1,
			UserID:           1,
			Symbol:           "MSFT",
			BuyDate:          ToDateTime(time.RFC3339, "2013-09-04T00:00:00Z"),
			BuyPrice:         ToDecimal("30.00"),
			Shares:           20,
			IsWatched:        false,
			TStopPercent:     ToNullDecimal("25.00"),
			BuyStopPrice:     DecimalNull,
			SellStopPrice:    DecimalNull,
			RisePercent:      DecimalNull,
			FallPercent:      DecimalNull,
			NotifyTStop:      true,
			NotifyBuyStop:    false,
			NotifySellStop:   false,
			NotifyRise:       false,
			NotifyFall:       false,
			NotifyBullBear:   false,
			LastTimeTStop:    ToNullDateTime(time.RFC3339, "2013-12-30T14:16:32-06:00"),
			LastTimeBuyStop:  DateTimeNull,
			LastTimeSellStop: DateTimeNull,
			LastTimeRise:     DateTimeNull,
			LastTimeFall:     DateTimeNull,
			LastTimeBullBear: DateTimeNull,
		},
		Detail: Detail{
			LastCloseDate:   ToNullDateTime(time.RFC3339, "2013-12-27T00:00:00-05:00"),
			TStopPrice:      ToNullDecimal("29.20"),
			Avg200Day:       ToNullFloat64("33.644428"),
			Avg50Day:        ToNullFloat64("36.832549"),
			SMAPercent:      ToNullFloat64("9.475926"),
			GainLossPercent: ToNullFloat64("24.433333"),
			GainLossDollar:  ToNullDecimal("146.60"),
		},
		CurrHour:  ToNullDateTime(time.RFC3339, "2013-12-30T14:00:00-06:00"),
		CurrPrice: ToNullDecimal("37.33"),
	}

	j, err := json.Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}

	if string(j) != `{"Stock":{"StockID":1,"UserID":1,"Symbol":"MSFT","BuyDate":"2013-09-04T00:00:00Z","BuyPrice":"30.00","Shares":20,"IsWatched":false,"TStopPercent":"25.00","BuyStopPrice":null,"SellStopPrice":null,"RisePercent":null,"FallPercent":null,"NotifyTStop":true,"NotifyBuyStop":false,"NotifySellStop":false,"NotifyRise":false,"NotifyFall":false,"NotifyBullBear":false,"LastTimeTStop":"2013-12-30T14:16:32-06:00","LastTimeBuyStop":null,"LastTimeSellStop":null,"LastTimeRise":null,"LastTimeFall":null,"LastTimeBullBear":null},"Detail":{"LastCloseDate":"2013-12-27T00:00:00-05:00","TStopPrice":"29.20","Avg200Day":"33.644428","Avg50Day":"36.832549","SMAPercent":"9.475926","GainLossPercent":"24.433333","GainLossDollar":"146.60"},"CurrHour":"2013-12-30T14:00:00-06:00","CurrPrice":"37.33"}` {
		fmt.Printf("%s\n", j)
		t.Fatal(fmt.Errorf("JSON does not match expected"))
	}
}

func TestJSONUnmarshal(t *testing.T) {
	j := `{
    "Stock": {
        "StockID": 1,
        "UserID": 1,
        "Symbol": "MSFT",
        "BuyDate": "2013-09-04T00:00:00Z",
        "BuyPrice": "30.00",
        "Shares": 20,
        "IsWatched": false,
        "TStopPercent": "25.00",
        "BuyStopPrice": null,
        "SellStopPrice": null,
        "RisePercent": null,
        "FallPercent": null,
        "NotifyTStop": true,
        "NotifyBuyStop": false,
        "NotifySellStop": false,
        "NotifyRise": false,
        "NotifyFall": false,
        "NotifyBullBear": false,
        "LastTimeTStop": "2013-12-30T14:16:32-06:00",
        "LastTimeBuyStop": null,
        "LastTimeSellStop": null,
        "LastTimeRise": null,
        "LastTimeFall": null,
        "LastTimeBullBear": null
    },
    "Detail": {
        "LastCloseDate": "2013-12-27T00:00:00-05:00",
        "TStopPrice": "29.20",
        "Avg200Day": "33.644428",
        "Avg50Day": "36.832549",
        "SMAPercent": "9.475926",
        "GainLossPercent": "24.433333",
        "GainLossDollar": "146.60"
    },
    "CurrHour": "2013-12-30T14:00:00-06:00",
    "CurrPrice": "37.33"
}`

	v := StockDetail{}
	err := json.Unmarshal([]byte(j), &v)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%+v\n", v)
}
