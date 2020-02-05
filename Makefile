build:
	go build .

dockerbuild:
	docker build -t c4po/harbor-exporter .

dockerpush:
	docker push c4po/harbor-exporter
