package graphql

import (
	"github.com/graphql-go/graphql"
	"golang.org/x/net/context"
)

type Service interface {
	Do(ctx context.Context, query string) *graphql.Result
}

type service struct {
	schema graphql.Schema
}

func NewService(schema graphql.Schema) Service {
	return &service{
		schema: schema,
	}
}

func (s *service) Do(ctx context.Context, query string) *graphql.Result {
	params := graphql.Params{
		Context: ctx,
		Schema: s.schema,
		RequestString: query,
	}
	return graphql.Do(params)
}
