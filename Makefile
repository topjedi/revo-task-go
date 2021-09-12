build:
	go build -o ./bin/test_app cmd/multi-ping/main.go

run: build
	./bin/test_app

build_image:
	 docker build . -t test_app

 container_run:
	 docker run -p 80:80 --env-file .env test_app

hot_start: build_image
	docker run -p 80:80 --env-file .env test_app