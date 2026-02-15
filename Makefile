.PHONY: go-cache-clean
go-cache-clean:
	go clean -cache && go clean -modcache

.PHONY: go-vendor
go-vendor:
	go mod tidy && go mod vendor && go mod verify

