IMAGE = gcr.io/${GOOGLE_CLOUD_PROJECT}/pizzaapp:$(shell git describe --tags --abbrev=0)
push:
	CGO_ENABLED=0 go build -tags netgo -ldflags "-w -extldflags \"-static\"" -o pizzaapp .
	docker build -t ${IMAGE} .
	gcloud docker -- push ${IMAGE}
	gcloud run deploy pizzaapp --image  ${IMAGE} --platform=managed --region=us-central1 \
		--allow-unauthenticated --set-env-vars=GOOGLE_RUN=true --max-instances=3 --concurrency=10 \
		--cpu=1 --memory=128