package internal

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

type DataAdapter struct {
	Dynamo *dynamo.DB
}

func (d *DataAdapter) New() *dynamo.DB {
	// TODO: point to AWS instead of local development dynamodb instance.
	d.Dynamo = dynamo.New(session.New(), &aws.Config{Endpoint: aws.String("http://localhost:4566"), Region: aws.String("us-west-2")})
	return d.Dynamo
}
