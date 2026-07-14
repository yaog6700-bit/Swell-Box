APP=swellbox
PKG=./cmd/swellbox

.PHONY: tidy build build-win run

tidy:
	go mod tidy

build:
	go build -o $(APP) $(PKG)

build-win:
	go build -ldflags "-H windowsgui" -o $(APP).exe $(PKG)

run: build
	./$(APP)
