.MAIN: sfu

sfu:
	docker stop sfu || true
	docker rm sfu || true

	docker build -t sfu -f sfu.dockerfile .
	docker run \
		--name sfu \
		--network="host" \
		-e AUDIT_LEVEL=1 \
		-v /tmp:/tmp \
		-v `pwd`/flexatars:/mnt/flexatars \
		sfu &

