# tfstore

Create a Terraform remote-state backend (an S3 bucket, versioned and
encrypted) via a single CloudFormation stack — no DynamoDB table required.
State locking uses [Terraform's native S3 locking](https://developer.hashicorp.com/terraform/language/backend/s3#state-locking)
(`use_lockfile = true`), so there is nothing else to provision or pay for.

## Requirements

- Terraform >= 1.11 (native S3 locking, `use_lockfile`, was introduced in 1.11)
- AWS credentials available via the standard SDK chain (env vars, shared
  config/credentials files, SSO, an instance/task role, etc.)

## Install

### Homebrew (macOS, arm64)

```bash
brew install youyo/tap/tfstore
```

This installs the `tfstore` binary and its zsh completion script.

### Manual download

Prebuilt binaries for **linux/arm64** and **darwin/arm64** are published on
the [GitHub releases page](https://github.com/youyo/tfstore/releases) —
amd64 is not built. `tfstore` ships as an unsigned binary, so if you download
it manually (not via Homebrew) rather than build it yourself, macOS
Gatekeeper quarantines it on first run. Clear the quarantine attribute once
before running it:

```bash
xattr -dr com.apple.quarantine ./tfstore
```

## Usage

```bash
$ tfstore
Creating stack...

bucket: tfstore-bucket-xxxxxxxxxxx
region: ap-northeast-1
key:    terraform.tfstate

Terraform initialize command is

terraform init \
  -backend-config 'bucket=tfstore-bucket-xxxxxxxxxxx' \
  -backend-config 'key=terraform.tfstate' \
  -backend-config 'region=ap-northeast-1' \
  -backend-config 'encrypt=true' \
  -backend-config 'use_lockfile=true'

$ terraform init \
  -backend-config 'bucket=tfstore-bucket-xxxxxxxxxxx' \
  -backend-config 'key=terraform.tfstate' \
  -backend-config 'region=ap-northeast-1' \
  -backend-config 'encrypt=true' \
  -backend-config 'use_lockfile=true'
```

### Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--stack-name` | `-n` | `tfstore` | CloudFormation stack name |
| `--region` | `-r` | resolved from AWS configuration | AWS region |
| `--key` | `-k` | `terraform.tfstate` | Terraform state object key |

```bash
$ tfstore --stack-name custom-stack-name --region us-east-1 --key envs/prod/terraform.tfstate
```

If a stack with the given name already exists, `tfstore` exits with an
error — it is a create-only tool and does not update or migrate an existing
stack.

## IAM policy for Terraform

Terraform's native S3 locking writes a companion `<key>.tflock` object next
to the state object, so the identity running `terraform init`/`plan`/`apply`
needs access to both objects plus `s3:ListBucket` on the bucket:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "TerraformStateObject",
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": [
        "arn:aws:s3:::<BucketName>/<key>",
        "arn:aws:s3:::<BucketName>/<key>.tflock"
      ]
    },
    {
      "Sid": "TerraformStateBucket",
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::<BucketName>"
    }
  ]
}
```

Replace `<BucketName>` and `<key>` with the values `tfstore` printed above.

## Author

[youyo](https://github.com/youyo)
