# tfstore

Create standard backend (S3+Dyanamodb) for terraform state files.
https://www.terraform.io/docs/backends/types/s3.html

## Install

- Homebrew

```
$ brew tap youyo/tap
$ brew install awslogin
```

Other platforms are download from [github release page](https://github.com/youyo/tfstore/releases).

## Usage

- simple way

```bash
$ tfstore
```

- specific StackName

```bash
$ tfstore --stack-name custom-stack-name
```

## Author

[youyo](https://github.com/youyo)
