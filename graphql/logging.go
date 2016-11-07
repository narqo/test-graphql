package graphql

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/graphql-go/graphql"
	"golang.org/x/net/context"
)

type loggingService struct {
	logger log.Logger
	Service
}

func NewLoggingService(logger log.Logger, s Service) Service {
	return &loggingService{logger, s}
}

func (s *loggingService) Do(ctx context.Context, query string) (res *graphql.Result) {
	defer func(begin time.Time) {
		var err error
		if len(res.Errors) > 0 {
			err = fmt.Errorf("request error: %v", res.Errors)
		}
		s.logger.Log(
			"method", "do",
			"query", query,
			"took", time.Since(begin),
			"error", err,
		)
	}(time.Now())
	res = s.Service.Do(ctx, query)
	return
}
