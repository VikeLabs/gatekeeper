package internal

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-redis/redis/v8"
	"github.com/guregu/dynamo"
)

type DataAdapter struct {
	Dynamo *dynamo.DB
	redis  *redis.Client
}

func (d *DataAdapter) New() {
	// TODO: point to AWS instead of local development dynamodb instance.
	d.Dynamo = dynamo.New(session.New(), &aws.Config{Endpoint: aws.String("http://localhost:4566"), Region: aws.String("us-west-2")})

	// TODO: point to actual Redis.
	d.redis = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}
