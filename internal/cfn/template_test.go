package cfn

import (
	"strings"
	"testing"
)

func TestTemplate_NotEmpty(t *testing.T) {
	if strings.TrimSpace(Template) == "" {
		t.Fatal("Template is empty, want the embedded CloudFormation template body")
	}
}

func TestTemplate_ContainsS3Bucket(t *testing.T) {
	if !strings.Contains(Template, "AWS::S3::Bucket") {
		t.Error("Template does not declare an AWS::S3::Bucket resource")
	}
}

func TestTemplate_ContainsPublicAccessBlockConfiguration(t *testing.T) {
	if !strings.Contains(Template, "PublicAccessBlockConfiguration") {
		t.Error("Template does not configure PublicAccessBlockConfiguration on the bucket")
	}
}

func TestTemplate_BucketRetainedOnDelete(t *testing.T) {
	if !strings.Contains(Template, "DeletionPolicy: Retain") {
		t.Error("Template does not set DeletionPolicy: Retain on the bucket")
	}
}

func TestTemplate_DoesNotContainDynamoDB(t *testing.T) {
	if strings.Contains(Template, "AWS::DynamoDB::Table") {
		t.Error("Template still declares an AWS::DynamoDB::Table resource; DynamoDB is no longer required")
	}
}

func TestTemplate_DoesNotUseTransform(t *testing.T) {
	if strings.Contains(Template, "Transform") {
		t.Error("Template declares a Transform section, which is not expected")
	}
}
