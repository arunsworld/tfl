TARGETS := ./cmd/tfl
LD_FLAGS := -s -w

build:
	pack build arunsworld/tfl:latest \
		--default-process tfl \
		--env "BP_GO_TARGETS=${TARGETS}" \
		--env "BP_GO_BUILD_LDFLAGS=${LD_FLAGS}" \
		--buildpack gcr.io/paketo-buildpacks/go \
		--builder paketobuildpacks/builder:tiny

push:
	docker push arunsworld/tfl:latest

run:
	docker run --rm -d -p 6133:80 arunsworld/tfl:latest -port=80
