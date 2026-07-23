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

func TestTemplate_DefaultsBucketNameFromNameAccountRegion(t *testing.T) {
	if !strings.Contains(Template, "tfstate-${Name}-${AWS::AccountId}-${AWS::Region}") {
		t.Error("Template does not build the default bucket name from Name/AWS::AccountId/AWS::Region")
	}
}

func TestTemplate_HasBucketNameOverrideCondition(t *testing.T) {
	if !strings.Contains(Template, "Conditions:") {
		t.Error("Template does not declare a Conditions section")
	}
	if !strings.Contains(Template, "BucketNameProvided") {
		t.Error("Template does not declare a BucketNameProvided condition")
	}
}

func TestTemplate_ParameterShape(t *testing.T) {
	nameBlockStart := strings.Index(Template, "\n  Name:")
	if nameBlockStart == -1 {
		t.Fatal("Template does not declare a Name parameter")
	}
	bucketNameBlockStart := strings.Index(Template, "\n  BucketName:")
	if bucketNameBlockStart == -1 {
		t.Fatal("Template does not declare a BucketName parameter")
	}

	// The Name parameter block runs from its header to the next
	// top-level parameter (BucketName) — it must not contain a Default
	// line, since Name is always supplied by the caller.
	nameBlockEnd := bucketNameBlockStart
	if bucketNameBlockStart < nameBlockStart {
		nameBlockEnd = len(Template)
	}
	nameBlock := Template[nameBlockStart:nameBlockEnd]
	if strings.Contains(nameBlock, "Default") {
		t.Error("Name parameter block must not declare a Default")
	}

	if !strings.Contains(Template, "BucketName:\n    Type: String\n    Default: ''") {
		t.Error("BucketName parameter must be Type: String with Default: ''")
	}

	if !strings.Contains(Template, "Fn::If") && !strings.Contains(Template, "!If") {
		t.Error("Template does not resolve BucketName via Fn::If/!If")
	}
	if !strings.Contains(Template, "BucketNameProvided") {
		t.Error("BucketName resolution does not reference the BucketNameProvided condition")
	}
}
