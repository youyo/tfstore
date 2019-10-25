package tfstore

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

const (
	TfStateKey string = "terraform.tfstate"
	Encrypt    string = "true"
)

// TfStore
type TfStore struct {
	Session           *session.Session
	Region            string
	BucketName        string
	DynamodbTableName string
}

// New
func New() *TfStore {
	sess := session.Must(
		session.NewSessionWithOptions(
			session.Options{
				SharedConfigState:       session.SharedConfigEnable,
				AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
			},
		),
	)

	tf := &TfStore{
		Session: sess,
		Region:  *sess.Config.Region,
	}

	return tf
}

// Create
func (tf *TfStore) Create(ctx context.Context, stackName string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	input := &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(Template),
		Capabilities: []*string{aws.String("CAPABILITY_AUTO_EXPAND")},
	}

	client := cloudformation.New(tf.Session)

	if _, err := client.CreateStackWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

// Wait
func (tf *TfStore) WaitCreation(ctx context.Context, stackName string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	input := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	client := cloudformation.New(tf.Session)

	if err := client.WaitUntilStackCreateCompleteWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

// GetOutput
func (tf *TfStore) GetOutput(ctx context.Context, stackName string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	input := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	client := cloudformation.New(tf.Session)

	resp, err := client.DescribeStacksWithContext(ctx, input)
	if err != nil {
		return err
	}

	for _, stack := range resp.Stacks {
		for _, output := range stack.Outputs {
			switch *output.OutputKey {
			case "BucketName":
				tf.BucketName = *output.OutputValue
			case "DynamodbTableName":
				tf.DynamodbTableName = *output.OutputValue
			default:
				continue
			}
		}
	}

	if tf.BucketName == "" || tf.DynamodbTableName == "" {
		return errors.New("failed to get outputs")
	}

	return nil
}

// GenerateCommandExample
func (tf *TfStore) GenerateCommandExample(ctx context.Context) string {
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	command := "terraform init -backend-config 'bucket=" + tf.BucketName + "' " +
		"-backend-config 'dynamodb_table=" + tf.DynamodbTableName + "' " +
		"-backend-config 'key=terraform.tfstate' " +
		"-backend-config 'region=" + tf.Region + "' " +
		"-backend-config 'encrypt=true'"

	return command
}
