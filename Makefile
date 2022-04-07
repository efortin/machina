.PHONY: all codesign tomd5

all:  tomd5 build install codesign

BINARY_NAME=machina

ARCH = $(shell /usr/bin/arch)

PREV_BUILD_HASH = $(shell cat .fingerprint || echo "")
BUILD_HASH = $(shell find . -type f -name "*.go" -exec md5sum {} + | md5sum | cut -c -32)



codesign:
	codesign --entitlements vz.entitlements -s - ${BINARY_NAME}-${ARCH}
	codesign --entitlements vz.entitlements -s - /usr/local/bin/machina

install:
	@echo ${ARCH}
	sudo cp ${BINARY_NAME}-${ARCH} /usr/local/bin/${BINARY_NAME}

build:
ifneq ($(BUILD_HASH), $(PREV_BUILD_HASH))
	echo $(BUILD_HASH) > .fingerprint
	GOARCH=${ARCH} GOOS=darwin go build -a -ldflags '-extldflags "-static"' -v -o ${BINARY_NAME}-${ARCH} main.go
	GOARCH=amd64 GOOS=darwin go build -a -ldflags '-extldflags "-static"' -v -o ${BINARY_NAME}-darwin main.go
else
	@echo "Matches previous build hash, skipping"
endif

clean:
	rm ${BINARY_NAME}-darwin
	rm ${BINARY_NAME}-linux
	rm ${BINARY_NAME}-windows
