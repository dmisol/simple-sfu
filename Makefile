.MAIN: build
### docker network create --driver='bridge' --subnet=78.159.236.156/24 dummy

build:
	@echo main
	rm main || true
	go build main.go

#		--network-alias=["78.159.236.156"] 

sfu:
	docker stop sfu || true
	docker rm sfu || true

	docker build -t sfu -f sfu.dockerfile .
	docker run \
		--name sfu \
		--network="host" \
		-e AUDIT_LEVEL=1 \
		-v /tmp:/tmp \
		sfu &
#	docker network connect dummy front
#	docker container start front &

