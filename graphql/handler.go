package graphql

import (
	"encoding/json"
	"net/http"
	"errors"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	kithttp "github.com/go-kit/kit/transport/http"
	kitracing "github.com/go-kit/kit/tracing/opentracing"
	"golang.org/x/net/context"
	"github.com/opentracing/opentracing-go"
	"time"
)

func MakeHandler(ctx context.Context, gqs Service, tracer opentracing.Tracer, logger log.Logger) http.Handler {
	opts := []kithttp.ServerOption{
		kithttp.ServerErrorLogger(logger),
		kithttp.ServerErrorEncoder(encodeError),
	}

	var graphqlEnpoint endpoint.Endpoint
	{
		graphqlLogger := log.NewContext(logger).With("method", "Graphql")

		graphqlEnpoint = makeGraphqlEndpoint(gqs)
		graphqlEnpoint = kitracing.TraceServer(tracer, "Graphql")(graphqlEnpoint)
		graphqlEnpoint = makeLoggingGraphqlEndpoint(graphqlLogger)(graphqlEnpoint)
	}

	graphqlHandler := kithttp.NewServer(
		ctx,
		graphqlEnpoint,
		decodeGraphqlRequest,
		encodeResponse,
		opts...,
	)

	return graphqlHandler
}

type graphqlRequest struct {
	query string
}

func makeGraphqlEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(graphqlRequest)
		res := s.Do(ctx, req.query)
		return res, nil
	}
}

func makeLoggingGraphqlEndpoint(logger log.Logger) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			defer func(begin time.Time) {
				logger.Log("error", err, "took", time.Since(begin))
			}(time.Now())
			return next(ctx, request)
		}
	}
}

var errBadRequest = errors.New("bad request")

func decodeGraphqlRequest(_ context.Context, r *http.Request) (interface{}, error) {
	query := r.URL.Query()["query"][0]
	if query == "" {
		return nil, errBadRequest
	}
	return graphqlRequest{
		query: query,
	}, nil
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		encodeError(ctx, e.error(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

type errorer interface {
	error() error
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}
