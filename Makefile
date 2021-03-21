.PHONY: release
release:
	curl -sL https://raw.githubusercontent.com/radekg/git-release/master/git-release --output /tmp/git-release
	chmod +x /tmp/git-release
	/tmp/git-release --repository-path=$GOPATH/src/github.com/combust-labs/firebuild-mmds
	rm -rf /tmp/git-release