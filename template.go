package tfstore

const Template string = `{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Transform": "AWS::Serverless-2016-10-31",
  "Description": "Terraform state store",
  "Parameters": {
    "BucketNoncurrentVersionExpirationInDays": {
      "Type": "Number",
      "Default": 731
    }
  },
  "Resources": {
    "Bucket": {
      "Type": "AWS::S3::Bucket",
      "Properties": {
        "BucketEncryption": {
          "ServerSideEncryptionConfiguration": [
            {
              "ServerSideEncryptionByDefault": {
                "SSEAlgorithm": "AES256"
              }
            }
          ]
        },
        "VersioningConfiguration": {
          "Status": "Enabled"
        },
        "LifecycleConfiguration": {
          "Rules": [
            {
              "NoncurrentVersionExpirationInDays": { "Ref" : "BucketNoncurrentVersionExpirationInDays" },
              "Status": "Enabled"
            }
          ]
        }
      }
    },
    "DynamodbTable": {
      "Type": "AWS::DynamoDB::Table",
      "Properties": {
        "AttributeDefinitions": [
          {
            "AttributeName": "LockID",
            "AttributeType": "S"
          }
        ],
        "KeySchema": [
          {
            "AttributeName": "LockID",
            "KeyType": "HASH"
          }
        ],
        "BillingMode": "PAY_PER_REQUEST",
        "SSESpecification": {
          "SSEEnabled": true
        }
      }
    }
  },
  "Outputs": {
    "BucketName": {
      "Value": { "Ref" : "Bucket" }
    },
    "DynamodbTableName": {
      "Value": { "Ref" : "DynamodbTable" }
    }
  }
}`
