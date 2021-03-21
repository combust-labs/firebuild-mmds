build-vminit:
	GOOS=linux CGO_ENABLED=0 installsuffix=cgo go build -o ./cmd/vminit/vminit-linux ./cmd/vminit/main.go
	
.PHONY: release
release:
	curl -sL https://raw.githubusercontent.com/radekg/git-release/master/git-release --output /tmp/git-release
	chmod +x /tmp/git-release
	/tmp/git-release --repository-path=${GOPATH}/src/github.com/combust-labs/firebuild-mmds
	rm -rf /tmp/git-release