package yql

// general stuff:
import "fmt"
import "log"
import "errors"
import "encoding/json"
import "strings"
import "reflect"

// networking:
import "io/ioutil"
import "net/http"
import "net/url"

func yqlValidateResultsType(results interface{}) (structType reflect.Type) {
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

func yqlExtractResponse(body []byte, results interface{}, structType reflect.Type) (err error) {
	if structType == nil {
		structType = yqlValidateResultsType(results)
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

type yqlResponse struct {
	Query struct {
		Count       int                    `json:"count"`
		CreatedDate string                 `json:"created"`
		Results     map[string]interface{} `json:"results"`
	} `json:"query"`
}

// `q` is the YQL query
func Get(results interface{}, q string) (err error) {
	// Validate type of `results`:
	structType := yqlValidateResultsType(results)

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
	err = yqlExtractResponse(body, results, structType)
	if err != nil {
		// debugging info:
		log.Printf("query:    %s\n", q)
		return
	}

	return
}
