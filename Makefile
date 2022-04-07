.PHONY: all codesign tomd5 build

all:  tomd5 build install codesign

BINARY_NAME=machina

ARCH = $(shell /usr/bin/arch)

PREV_BUILD_HASH = $(shell cat .fingerprint || echo "")
BUILD_HASH = $(shell find . -type f -name "*.go" -o -name "go.mod" -exec md5sum {} + | md5sum | cut -c -32)



codesign:
	codesign --entitlements vz.entitlements -s - ${BINARY_NAME}-${ARCH}
	codesign --entitlements vz.entitlements -s - /usr/local/bin/machina

install:
	@echo ${ARCH}
	sudo cp build/${BINARY_NAME}-${ARCH} /usr/local/bin/${BINARY_NAME}

build:
ifneq ($(BUILD_HASH), $(PREV_BUILD_HASH))
	echo $(BUILD_HASH) > .fingerprint
	GOARCH=${ARCH} GOOS=darwin go build -a -ldflags '-extldflags "-static"' -v -o build/${BINARY_NAME}-${ARCH} main.go
else
	@echo "Matches previous build hash, skipping"
endif

clean:
	rm .fingerprint
	rm build/${BINARY_NAME}-${ARCH}
