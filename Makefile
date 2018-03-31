CONTROLLER_IMAGE = xychu/throttle-controller
CONTROLLER_TAG = v1
CONTROLLER_BIN = throttle
WEBHOOK_IMAGE = xychu/throttle-admission-webhook
WEBHOOK_TAG = v1
WEBHOOK_BIN = webhook

build-ctr:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(CONTROLLER_BIN) .
	docker build --no-cache -t $(CONTROLLER_IMAGE):$(CONTROLLER_TAG) .
	rm -rf $(CONTROLLER_BIN)

push-ctr:
	docker push $(CONTROLLER_IMAGE):$(CONTROLLER_TAG)

build-webhook:
	cd webhook; CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(WEBHOOK_BIN) .
	docker build --no-cache -t $(WEBHOOK_IMAGE):$(WEBHOOK_TAG) ./webhook/
	rm -rf ./webhook/$(WEBHOOK_BIN)

push-webhook:
	docker push $(WEBHOOK_IMAGE):$(WEBHOOK_TAG)

build-all: build-webhook build-ctr

push-all: push-webhook push-ctr
