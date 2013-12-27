package yql

// general stuff:
import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"time"
)

// networking:
import "io/ioutil"
import "net/http"
import "net/url"

// ------------- private structures:

const dateFmt = "2006-01-02"

// Head to http://developer.yahoo.com/yql/console/?q=select%20*%20from%20yahoo.finance.quote%20where%20symbol%20in%20(%22YHOO%22%2C%22AAPL%22%2C%22GOOG%22%2C%22MSFT%22)&env=store%3A%2F%2Fdatatables.org%2Falltableswithkeys
// to understand this JSON structure.

type yqlResponse struct {
	Query struct {
		Count       int    `json:"count"`
		CreatedDate string `json:"created"`
		// TODO(jsd): Use `*json.RawMessage` type instead.
		Results map[string]interface{} `json:"results"`
	} `json:"query"`
}

type History struct {
	Symbol string
	Date   string
	Open   string
	Close  string
	High   string
	Low    string
	Volume string
}

func validateResultsType(results interface{}) (structType reflect.Type) {
	if results == nil {
		panic(errors.New("results cannot be nil"))
	}
	rt := reflect.TypeOf(results)
	if rt.Kind() != reflect.Ptr {
		panic(errors.New("results must be a pointer"))
	}
	rtp := rt.Elem()
	if rtp.Kind() != reflect.Slice {
		panic(errors.New("results must be a pointer to a slice"))
	}
	structType = rtp.Elem()
	if structType.Kind() != reflect.Struct {
		panic(errors.New("results must be a pointer to a slice of structs"))
	}
	return
}

func extractResponse(body []byte, results interface{}, structType reflect.Type) (err error) {
	if structType == nil {
		structType = validateResultsType(results)
	}

	// results is now guaranteed to be a pointer to a slice of structs.
	sliceValue := reflect.ValueOf(results).Elem()

	// decode JSON response body:
	yrsp := new(yqlResponse)
	err = json.Unmarshal(body, yrsp)
	if err != nil {
		// debugging info:
		log.Printf("response: %s\n", body)
		return
	}

	// Decode the Results map as either an array of objects or a single object:
	quote := yrsp.Query.Results["quote"]
	if quote == nil {
		// TODO(jsd): clear the sliceValue pointer to nil?
		return
	}

	switch t := quote.(type) {
	default:
		panic(errors.New("unexpected JSON result type for 'quote'"))
	case []interface{}:
		sl := sliceValue
		for j, n := range t {
			// Append to the slice for each array element:
			m := n.(map[string]interface{})
			sl = reflect.Append(sl, reflect.Zero(structType))
			el := sl.Index(j)
			for i := 0; i < structType.NumField(); i++ {
				f := structType.Field(i)
				if v, ok := m[f.Name]; ok {
					el.Field(i).Set(reflect.ValueOf(v))
				}
			}
		}
		sliceValue.Set(sl)
	case map[string]interface{}:
		// Insert the only element of the slice:
		sl := reflect.Append(sliceValue, reflect.Zero(structType))
		el0 := sl.Index(0)
		for i := 0; i < structType.NumField(); i++ {
			f := structType.Field(i)
			if v, ok := t[f.Name]; ok {
				el0.Field(i).Set(reflect.ValueOf(v))
			}
		}
		sliceValue.Set(sl)
	}

	return
}

// `q` is the YQL query
func Get(results interface{}, q string) (err error) {
	// Validate type of `results`:
	structType := validateResultsType(results)

	// form the YQL URL:
	u := `http://query.yahooapis.com/v1/public/yql?q=` + url.QueryEscape(q) + `&format=json&env=store%3A%2F%2Fdatatables.org%2Falltableswithkeys`
	resp, err := http.Get(u)
	if err != nil {
		return
	}

	// read body:
	defer resp.Body.Close()

	// Need a 200 response:
	if resp.StatusCode != 200 {
		err = fmt.Errorf("%s", resp.Status)
		return
	}
	if hp, ok := resp.Header["Content-Type"]; ok && len(hp) > 0 {
		if strings.Split(hp[0], ";")[0] != "application/json" {
			err = fmt.Errorf("Expected JSON content-type: %s", hp[0])
			return
		}
	}

	// Read the whole response body into memory:
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// Extract the unstable JSON structure's results field as an array:
	err = extractResponse(body, results, structType)
	if err != nil {
		// debugging info:
		log.Printf("query:    %s\n", q)
		return
	}

	return
}

// Single query result:
type yearQueryResult struct {
	Year    int
	Error   error
	History *[]History
}

// Sortable list of query results:
type yearQueryResultList struct {
	items []yearQueryResult
}

// Len is the number of elements in the collection.
func (l *yearQueryResultList) Len() int { return len(l.items) }

// Less reports whether the element with
// index i should sort before the element with index j.
// YQL reports dates in descending order.
func (l *yearQueryResultList) Less(i, j int) bool {
	return l.items[i].Year > l.items[j].Year
}

// Swap swaps the elements with indexes i and j.
func (l *yearQueryResultList) Swap(i, j int) {
	l.items[i], l.items[j] = l.items[j], l.items[i]
}

type quote struct {
	Symbol             string
	LastTradePriceOnly string
}

// Gets the current trading price for a symbol.
func GetCurrent(symbol string) (price *big.Rat, err error) {
	quot := make([]quote, 0, 1)
	query := fmt.Sprintf(`select LastTradePriceOnly from yahoo.finance.quote where symbol = "%s"`, symbol)
	err = Get(&quot, query)
	if err != nil {
		return
	}

	// Nothing?
	if len(quot) == 0 {
		return nil, nil
	}

	price = new(big.Rat)
	price.SetString(quot[0].LastTradePriceOnly)
	return price, nil
}

// Gets all historical data for a symbol between startDate and endDate.
func GetHistory(symbol string, startDate, endDate time.Time) (results []History, err error) {
	// NOTE(jsd): YQL queries over stocks only respond to queries requesting up to 365 date records; results is nil otherwise.
	days := int(endDate.Sub(startDate) / (time.Duration(24) * time.Hour))

	// TODO(jsd): Subtract weekend dates (and holidays).
	results = make([]History, 0, days)

	count := days / 365
	if (days % 365) > 0 {
		count++
	}

	// Submit a batch of queries to pull all the data year by year:
	queries := make([]string, 0, count)
	date := startDate
	for year := 0; year < count; year++ {
		qstartDate, qendDate := date, date.Add(time.Duration(364*24)*time.Hour)
		if qendDate.After(endDate) {
			qendDate = endDate
		}

		// TODO(jsd): YQL parameter escaping!
		queries = append(
			queries,
			fmt.Sprintf(`select Symbol, Date, Open, Close, High, Low, Volume from yahoo.finance.historicaldata where symbol = "%s" and startDate = "%s" and endDate = "%s"`,
				symbol,
				qstartDate.Format(dateFmt),
				qendDate.Format(dateFmt),
			),
		)

		date = date.Add(time.Duration(365*24) * time.Hour)
	}

	// Run the queries in parallel:
	queryResults := make(chan yearQueryResult)
	for i, q := range queries {
		go func(i int, q string) {
			res := make([]History, 0, 364)

			err := Get(&res, q)
			if err != nil {
				queryResults <- yearQueryResult{
					Year:    i,
					Error:   err,
					History: nil,
				}
				return
			}

			queryResults <- yearQueryResult{
				Year:    i,
				History: &res,
			}
		}(i, q)
	}

	// Collect the query results:
	list := &yearQueryResultList{items: make([]yearQueryResult, 0, count)}
	for i := 0; i < count; i++ {
		r := <-queryResults
		list.items = append(list.items, r)
	}

	// Order list in descending order:
	sort.Sort(list)

	for _, r := range list.items {
		if r.History == nil {
			return nil, r.Error
		}

		for _, h := range *(r.History) {
			results = append(results, h)
		}
	}

	return
}
