.PHONEY: clean get

default: all

all: build

build: get
	 if [ ! -d "./bin/" ]; then mkdir ./bin/; fi
	 env GOOS=linux GOARCH=amd64 go build -v -o ./bin/lb_nic_order ./src/
get:
	 go get -d ./src/
clean:
	go clean
install: 
ifneq ($(shell uname),Linux)
	echo "Install only available on Linux"
	exit 1
endif
	cp ./bin/lb_nic_order /usr/local/sbin/
