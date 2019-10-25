# tfstore

Create standard backend (S3+Dyanamodb) for terraform state files.
https://www.terraform.io/docs/backends/types/s3.html

## Install

- Homebrew

```
$ brew tap youyo/tap
$ brew install tfstore
```

Other platforms are download from [github release page](https://github.com/youyo/tfstore/releases).

## Usage

- simple way

```bash
$ tfstore
-------------------------------------
Creating stack...

bucket: tfstore-bucket-xxxxxxxxxxx
dynamodb_table: tfstore-DynamodbTable-xxxxxxxxxxx
region: ap-northeast-1
key: terraform.tfstate
encrypt: true

Terraform initialize command is

terraform init -backend-config 'bucket=tfstore-bucket-xxxxxxxxxxx' -backend-config 'dynamodb_table=tfstore-DynamodbTable-xxxxxxxxxxx' -backend-config 'key=terraform.tfstate' -backend-config 'region=ap-northeast-1' -backend-config 'encrypt=true'
-------------------------------------

$ terraform init -backend-config 'bucket=tfstore-bucket-xxxxxxxxxxx' -backend-config 'dynamodb_table=tfstore-DynamodbTable-xxxxxxxxxxx' -backend-config 'key=terraform.tfstate' -backend-config 'region=ap-northeast-1' -backend-config 'encrypt=true'
```

- specific StackName

```bash
$ tfstore --stack-name custom-stack-name
.
.
.
```

## Author

[youyo](https://github.com/youyo)
