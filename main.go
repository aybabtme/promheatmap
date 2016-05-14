package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/aybabtme/uniplot/spark"
	"github.com/gonum/plot"
	"github.com/prometheus/client_golang/api/prometheus"
	"github.com/prometheus/common/model"
)

func main() {
	log.SetPrefix("promheatmap: ")
	log.SetFlags(0)
	var (
		cfg      prometheus.Config
		from     = 2 * time.Hour
		to       = 1 * time.Second
		step     = (from - to) / 360
		filename = "query.png"
		minVal   = 1.
		units    = "unitless"
	)
	flag.StringVar(&cfg.Address, "addr", "", "address of the Prometheus to connect to")
	flag.DurationVar(&from, "from", from, "while fetch metrics starting from that far in the past")
	flag.DurationVar(&to, "to", to, "while fetch metrics up to that far in the past")
	flag.DurationVar(&step, "step", step, "will fetch data points for every step")
	flag.StringVar(&filename, "out", filename, "where to save the plot to, including extension desired")
	flag.Float64Var(&minVal, "min", minVal, "smallest value to plot")
	flag.StringVar(&units, "units", units, "units to use, one of duration/bytes/unitless")
	flag.Parse()

	query := strings.Join(flag.Args(), " ")
	if query == "" {
		log.Fatal("no query specified")
	}

	var unitTicker func(plot.Ticker) plot.Ticker
	switch units {
	case "duration":
		unitTicker = readableDuration
	case "unitless":
		unitTicker = func(in plot.Ticker) plot.Ticker { return in }
	case "bytes":
		unitTicker = readableBytes
	}

	cfg.Transport = &peakTransport{wrap: prometheus.DefaultTransport}

	client, err := prometheus.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	api := prometheus.NewQueryAPI(client)

	ctx := context.Background()
	log.Printf("querying prometheus")
	v, err := api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(-from),
		End:   time.Now().Add(-to),
		Step:  step,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("received results")

	switch val := v.(type) {
	case *model.Scalar:
		log.Printf("scalar %v: %v", val.Timestamp, val.Value)
	case model.Vector:
		for _, scalar := range val {
			log.Printf("vector %v: %v", scalar.Timestamp, scalar.Value)
		}
	case model.Matrix:
		log.Printf("plotting %d points to %q", countMatrixPoints(val), filename)
		title := query
		if err := plotScatter(val, model.SampleValue(minVal), unitTicker, title, filename); err != nil {
			log.Fatal(err)
		}
	case *model.String:
		log.Printf("string %v: %v", val.Timestamp, val.Value)
	default:
		log.Fatalf("received type %T from prometheus", v)
	}
}

func countMatrixPoints(mat model.Matrix) (count int) {
	for _, str := range mat {
		count += len(str.Values)
	}
	return count
}

type peakTransport struct {
	wrap prometheus.CancelableTransport
}

func (tpt *peakTransport) CancelRequest(req *http.Request) {
	tpt.wrap.CancelRequest(req)
}

func (tpt *peakTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := tpt.wrap.RoundTrip(req)
	s := spark.ReaderOut(resp.Body, os.Stderr)
	resp.Body = &fnReadCloser{
		readFn:  s.Read,
		closeFn: resp.Body.Close,
	}
	return resp, err
}

type fnReadCloser struct {
	closeFn func() error
	readFn  func([]byte) (int, error)
}

func (fn *fnReadCloser) Read(p []byte) (int, error) { return fn.readFn(p) }
func (fn *fnReadCloser) Close() error               { return fn.closeFn() }
