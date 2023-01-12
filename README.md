## Developing

### Install tools
* https://github.com/cli/cli#installation
* https://helm.sh/docs/intro/install

```
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.0
go install github.com/google/ko@v0.11.2
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.8.0
go install github.com/stern/stern@latest
```

### Clone the repo
```sh
export GOPATH=${HOME}/workspaces/go # Override if desired
export PATH=$PATH:$GOPATH/bin
mkdir -p ${GOPATH}/src/github.com/ellistarn
cd ${GOPATH}/src/github.com/ellistarn
gh repo clone ellistarn/kube-event-bridge
cd kube-event-bridge
```

### Setup AWS Resources

```sh
AWS_ACCOUNT_ID=${AWS_ACCOUNT_ID:=undefined} # Recommended to be included in your bashrc
aws ecr create-repository --repository-name kube-event-bridge/controller --image-scanning-configuration scanOnPush=true
aws iam create-policy --policy-name kube-event-bridge --policy-document '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
          "events:*",
          "sqs:*"
      ],
      "Resource": "*"
    }
  ]
}'
eksctl create iamserviceaccount \
  --name kube-event-bridge \
  --role-name kube-event-bridge \
  --cluster $(kubectl config view --minify -o jsonpath='{.clusters[].name}' | rev | cut -d"/" -f1 | rev | cut -d"." -f1) \
  --attach-policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/kube-event-bridge \
  --role-only \
  --approve
```

### Install the controller and stream logs
```sh
make apply
stern -l app.kubernetes.io/name=kube-event-bridge
```

### Cleanup
```sh
eksctl delete iamserviceaccount --name kube-event-bridge --cluster $(kubectl config view --minify -o jsonpath='{.clusters[].name}' | rev | cut -d"/" -f1 | rev | cut -d"." -f1)
aws iam delete-policy --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/kube-event-bridge
```


## Demo

Remove this once we have the SQS controller
```
aws sqs receive-message --queue-url $(aws sqs get-queue-url --queue-name demo | jq -r ".QueueUrl")

aws sqs send-message --queue-url $(aws sqs get-queue-url --queue-name demo | jq -r ".QueueUrl") --message-body "$(echo $(k get events -ojson | jq ".items[0]"))"

aws sqs delete-message --queue-url demo --receipt-handle $(aws sqs receive-message --queue-url $(aws sqs get-queue-url --queue-name demo | jq -r ".QueueUrl") | jq -r ".Messages[].ReceiptHandle")
```
