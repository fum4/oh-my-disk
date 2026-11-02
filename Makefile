ROOT := $(CURDIR)
export GOPATH := $(ROOT)/.build/go
export GOMODCACHE := $(ROOT)/.build/gomod
export GOCACHE := $(ROOT)/.build/gocache
export GOBIN := $(ROOT)/.build/bin
export GOTOOLCHAIN := go1.26.10
# Keep unrelated host CGO libraries out of the app (especially universal builds).
export CGO_CFLAGS :=
export CGO_LDFLAGS :=
WAILS := $(GOBIN)/wails

.PHONY: app app-universal app-run cli check icon wails
wails:
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
icon:
	./scripts/icon.sh
app: wails icon
	./scripts/build-app.sh darwin/arm64
app-universal: wails icon
	./scripts/build-app.sh darwin/universal
app-run:
	open build/bin/oh-my-disk.app
cli:
	go build -o oh-my-disk .
check:
	go vet ./...
	go test ./...
