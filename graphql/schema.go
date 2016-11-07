package graphql

import (
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"golang.org/x/net/context"
)

type resolver struct {
	client *http.Client
	tracer opentracing.Tracer
}

func NewResolver(tracer opentracing.Tracer) *resolver {
	return &resolver{
		client: &http.Client{Transport: &nethttp.Transport{}},
		tracer: tracer,
	}
}

func (r *resolver) startSpanFromContext(ctx context.Context, name string) (opentracing.Span, context.Context) {
	var span opentracing.Span
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		span = r.tracer.StartSpan(name, opentracing.ChildOf(parentSpan.Context()))
	} else {
		span = r.tracer.StartSpan(name)
	}
	return span, opentracing.ContextWithSpan(ctx, span)
}

func (r *resolver) get(ctx context.Context, name, urlStr string) error {
	span, ctx := r.startSpanFromContext(ctx, name)
	defer span.Finish()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	req, ht := nethttp.TraceRequest(r.tracer, req)
	defer ht.Finish()

	resp, err := r.client.Do(req)
	if err != nil {
		span.SetTag(string(ext.Error), err.Error())
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (r *resolver) User(ctx context.Context, id string) (string, error) {
	if err := r.get(ctx, "user", "http://yandex.ru"); err != nil {
		return "", err
	}
	return "id:" + id, nil
}

func (r *resolver) UserName(ctx context.Context, id string) (string, error) {
	if err := r.get(ctx, "userName", "http://yandex.ru"); err != nil {
		return "", err
	}
	return "name:" + id, nil
}

func NewSchema(r *resolver) (graphql.Schema, error) {
	userType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: "User",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.String,
				},
				"name": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						if id, ok := p.Source.(string); ok {
							return r.UserName(p.Context, id)
						}
						return nil, nil
					},
				},
			},
		},
	)

	fields := graphql.Fields{
		"user": &graphql.Field{
			Description: "Search something",
			Type:        userType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				if id, ok := p.Args["id"].(string); ok {
					if user, err := r.User(p.Context, id); err != nil {
						return nil, err
					} else {
						return user, nil
					}
				}
				return nil, nil
			},
		},
	}

	rootQueryType := graphql.NewObject(
		graphql.ObjectConfig{
			Name:   "Query",
			Fields: fields,
		},
	)
	return graphql.NewSchema(graphql.SchemaConfig{
		Query: rootQueryType,
	})
}
