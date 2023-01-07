## Developing

### Install tools
* https://github.com/cli/cli#installation
* https://helm.sh/docs/intro/install
* `go install github.com/google/ko@v0.11.2`
* `go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.8.0`
* `go install github.com/stern/stern@latest`

### Clone the repo
```sh
export GOPATH=$HOME/workspaces/go # Override if desired
mkdir -p $GOPATH/src/github.com/ellistarn
cd $GOPATH/src/github.com/ellistarn
gh repo clone ellistarn/kube-event-bridge
cd kube-event-bridge
```

### Create an ECR Repository
```sh
aws ecr create-repository --repository-name kube-event-bridge/controller --image-scanning-configuration scanOnPush=true
```

### Install the controller and stream logs
```sh
make apply
stern -l app.kubernetes.io/name=kube-event-bridge
```
