# aws-billing-slack

## Requirements

[Installing the AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html)

## Build and Deploy

```sh
$ sam build
$ sam deploy --guided
```

## Delete Application

```sh
$ aws cloudformation delete-stack --stack-name <STACK_NAME>
```

Note: S3 buckets with prefix `aws-sam-cli-managed-default-samclisourcebucket` are not erased.

## Slack Screenshot

![image](https://user-images.githubusercontent.com/26246951/92502882-6e8dcb00-f23b-11ea-8b55-31d96358f14b.png)
