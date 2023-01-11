export KO_DOCKER_REPO = ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/kube-event-bridge

presubmit: verify ## Run these steps before commiting code

verify: ## Verify code. Includes codegen, dependencies, linting, formatting, etc
	go mod tidy
	go generate ./...
	go vet ./...
	golangci-lint run
	@git diff --quiet ||\
		{ echo "New file modification detected in the Git working tree. Please check in before commit."; git --no-pager diff --name-only | uniq | awk '{print "  - " $$0}'; \
		if [ "${CI}" == 'true' ]; then\
			exit 1;\
		fi;}

apply:
	helm upgrade --install kube-event-bridge ./chart \
	  --set controller.image=$$(ko build -B github.com/ellistarn/kube-event-bridge/cmd/controller) \
	  --set serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn=arn:aws:iam::${AWS_ACCOUNT_ID}:role/kube-event-bridge

delete:
	helm uninstall kube-event-bridge
