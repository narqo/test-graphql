package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"test-graphql/graphql"

	"github.com/go-kit/kit/log"
	"github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"golang.org/x/net/context"
)

const zipkinApiUrl = "http://localhost:9411/api/v1/spans"

func main() {
	addr := os.Getenv("PORT")
	debugAddr := os.Getenv("DEBUG_PORT")

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	var tracer opentracing.Tracer
	{
		logger := log.NewContext(logger).With("tracer", "Zipkin")
		logger.Log("addr", zipkinApiUrl)
		collector, err := zipkin.NewHTTPCollector(
			zipkinApiUrl,
			zipkin.HTTPLogger(logger),
		)
		if err != nil {
			logger.Log("msg", "unable to create Zipkin HTTP collector", "error", err)
			os.Exit(1)
		}
		tracer, err = zipkin.NewTracer(
			zipkin.NewRecorder(collector, true, "localhost:"+addr, "test-graphql"),
		)
		if err != nil {
			logger.Log("msg", "unable to create Zipkin tracer", "error", err)
			os.Exit(1)
		}
	}

	ctx := context.Background()

	var gqls graphql.Service
	{
		schema, err := graphql.NewSchema(
			graphql.NewResolver(tracer),
		)
		if err != nil {
			logger.Log("error", err)
			os.Exit(1)
		}
		gqls = graphql.NewService(schema)
		gqls = graphql.NewLoggingService(logger, gqls)
	}

	httpLogger := log.NewContext(logger).With("component", "http")

	mux := http.NewServeMux()
	mux.Handle("/graphql", graphql.MakeHandler(ctx, gqls, tracer, httpLogger))

	errc := make(chan error, 2)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errc <- fmt.Errorf("%s", <-c)
	}()
	// Debug listener
	go func() {
		logger := log.NewContext(logger).With("transport", "debug")

		m := http.NewServeMux()
		m.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		m.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		m.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		m.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		m.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

		debugAddr = ":" + debugAddr
		logger.Log("addr", debugAddr)
		errc <- http.ListenAndServe(debugAddr, m)
	}()
	// HTTP transport
	go func() {
		addr = ":" + addr
		logger.Log("transport", "http", "address", addr, "msg", "listening")
		errc <- http.ListenAndServe(addr, mux)
	}()

	logger.Log("terminated", <-errc)
}
