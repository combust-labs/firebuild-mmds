package injectors

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/hashicorp/go-hclog"
)

type testMetadataServer struct {
	mdPath string
}

func newTestServer(metadataPath string) *testMetadataServer {
	return &testMetadataServer{
		mdPath: metadataPath,
	}
}

func (srv *testMetadataServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/"+srv.mdPath {
		if r.Method == http.MethodGet {
			w.Write([]byte(testJsonData))
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func TestInjectEnvironment(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := "latest/meta-data"
	server := httptest.NewServer(newTestServer(mdPath))
	defer server.Close()

	mmdsData, err := mmds.GuestFetchMMDSMetadata(hclog.Default(), fmt.Sprintf("%s/%s", server.URL, mdPath))
	if err != nil {
		t.Fatal("expected fetch to succeed but received an error:", err)
	}

	file := filepath.Join(tempDir, "etc/profile.d/run-env.sh")
	if err := InjectEnvironment(hclog.Default(), mmdsData, file); err != nil {
		t.Fatal("expected the environment to be injected but received an error:", err)
	}
}

func TestInjectHostname(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := "latest/meta-data"
	server := httptest.NewServer(newTestServer(mdPath))
	defer server.Close()

	mmdsData, err := mmds.GuestFetchMMDSMetadata(hclog.Default(), fmt.Sprintf("%s/%s", server.URL, mdPath))
	if err != nil {
		t.Fatal("expected fetch to succeed but received an error:", err)
	}

	// the file must exist before writing:
	if err := os.Mkdir(filepath.Join(tempDir, "etc"), 0755); err != nil {
		t.Fatal("expected etc directory to be created:", err)
	}

	file := filepath.Join(tempDir, "etc/hostname")

	if _, err := os.Create(file); err != nil {
		t.Fatal("expected etc/hostname file to be created to exist:", err)
	}

	if err := InjectHostname(hclog.Default(), mmdsData, file); err != nil {
		t.Fatal("expected the hostname to be injected but received an error:", err)
	}
}

func TestInjectHostsWithInterfaces(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := "latest/meta-data"
	server := httptest.NewServer(newTestServer(mdPath))
	defer server.Close()

	mmdsData, err := mmds.GuestFetchMMDSMetadata(hclog.Default(), fmt.Sprintf("%s/%s", server.URL, mdPath))
	if err != nil {
		t.Fatal("expected fetch to succeed but received an error:", err)
	}

	// the file must exist before writing:
	if err := os.Mkdir(filepath.Join(tempDir, "etc"), 0755); err != nil {
		t.Fatal("expected etc directory to be created:", err)
	}

	file := filepath.Join(tempDir, "etc/hosts")

	if _, err := os.Create(file); err != nil {
		t.Fatal("expected etc/hosts file to be created to exist:", err)
	}

	defaultHosts := map[string]string{
		"127.0.0.1": "localhost",
		"::1":       "localhost ip6-localhost ip6-loopback",
		"fe00::0":   "ip6-localnet",
		"ff00::0":   "ip6-mcastprefix",
		"ff02::1":   "ip6-allnodes",
		"ff02::2":   "ip6-allrouters",
	}

	if err := InjectHosts(hclog.Default(), mmdsData, defaultHosts, file); err != nil {
		t.Fatal("expected the hosts to be injected but received an error:", err)
	}
	fileBytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal("expected the hosts file to be read but received an error:", err)
	}
	ok := false
	lines := strings.Split(string(fileBytes), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == fmt.Sprintf("%s\t%s", mmdsData.Network.Interfaces["c6:15:a7:48:76:16"].IP, mmdsData.LocalHostname) {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("hosts entry not found")
	}
}

func TestInjectHostsWithoutInterfaces(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := "latest/meta-data"
	server := httptest.NewServer(newTestServer(mdPath))
	defer server.Close()

	mmdsData, err := mmds.GuestFetchMMDSMetadata(hclog.Default(), fmt.Sprintf("%s/%s", server.URL, mdPath))
	if err != nil {
		t.Fatal("expected fetch to succeed but received an error:", err)
	}

	mmdsData.Network.Interfaces = map[string]*mmds.MMDSNetworkInterface{}

	// the file must exist before writing:
	if err := os.Mkdir(filepath.Join(tempDir, "etc"), 0755); err != nil {
		t.Fatal("expected etc directory to be created:", err)
	}

	file := filepath.Join(tempDir, "etc/hosts")

	if _, err := os.Create(file); err != nil {
		t.Fatal("expected etc/hosts file to be created to exist:", err)
	}

	defaultHosts := map[string]string{
		"127.0.0.1": "localhost",
		"::1":       "localhost ip6-localhost ip6-loopback",
		"fe00::0":   "ip6-localnet",
		"ff00::0":   "ip6-mcastprefix",
		"ff02::1":   "ip6-allnodes",
		"ff02::2":   "ip6-allrouters",
	}

	if err := InjectHosts(hclog.Default(), mmdsData, defaultHosts, file); err != nil {
		t.Fatal("expected the hosts to be injected but received an error:", err)
	}
	fileBytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal("expected the hosts file to be read but received an error:", err)
	}
	ok := false
	lines := strings.Split(string(fileBytes), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == fmt.Sprintf("127.0.0.1\tlocalhost %s", mmdsData.LocalHostname) {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("hosts entry not found")
	}
}

func TestInjectSSHKeys(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := "latest/meta-data"
	server := httptest.NewServer(newTestServer(mdPath))
	defer server.Close()

	mmdsData, err := mmds.GuestFetchMMDSMetadata(hclog.Default(), fmt.Sprintf("%s/%s", server.URL, mdPath))
	if err != nil {
		t.Fatal("expected fetch to succeed but received an error:", err)
	}

	// the file must exist before writing and we require specific permissions:
	if err := os.MkdirAll(filepath.Join(tempDir, "home/alpine/.ssh"), 0700); err != nil {
		t.Fatal("expected home/alpine/.ssh directory to be created:", err)
	}

	file := filepath.Join(tempDir, "home/alpine/.ssh/authorized_keys")

	if _, err := os.Create(file); err != nil {
		t.Fatal("expected home/alpine/.ssh/authorized_keys file to be created to exist:", err)
	}

	explicitMode := fs.FileMode(0400)

	if err := os.Chmod(file, explicitMode); err != nil {
		t.Fatal("expected home/alpine/.ssh/authorized_keys file chmod to succeed:", err)
	}

	if err := InjectSSHKeys(hclog.Default(), mmdsData, filepath.Join(tempDir, "home/%s/.ssh/authorized_keys")); err != nil {
		t.Fatal("expected the ssh keys to be injected but received an error:", err)
	}

	// mode must be changed back to the original one
	stat, err := os.Stat(file)
	if err != nil {
		t.Fatal("failed stat home/alpine/.ssh/authorized_keys:", err)
	}

	if stat.Mode().Perm() != explicitMode {
		t.Fatal("stat mode home/alpine/.ssh/authorized_keys not what expected")
	}

	fileBytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal("expected the hosts file to be read but received an error:", err)
	}

	if !strings.Contains(string(fileBytes), mmdsData.Users["alpine"].SSHKeys) {
		t.Fatal("home/alpine/.ssh/authorized_keys did not contain required ssh keys")
	}
}

func TestInjectEntrypoint(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	mdPath := "latest/meta-data"
	server := httptest.NewServer(newTestServer(mdPath))
	defer server.Close()

	mmdsData, err := mmds.GuestFetchMMDSMetadata(hclog.Default(), fmt.Sprintf("%s/%s", server.URL, mdPath))
	if err != nil {
		t.Fatal("expected fetch to succeed but received an error:", err)
	}

	newMMDSData := &mmds.MMDSData{
		EntrypointJSON: mmdsData.EntrypointJSON,
	}

	file := filepath.Join(tempDir, "usr/bin/firebuild-entrypoint.sh")

	if err := InjectEntrypoint(hclog.Default(), newMMDSData, file); err != nil {
		t.Fatal("expected the entrypoint runner to be injected but received an error:", err)
	}

	fileBytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal("expected the hosts file to be read but received an error:", err)
	}

	if string(fileBytes) != "#!/bin/sh\n\n/bin/sh -c 'ETCD_VERSION=\"3.4.0\"; cd / && /usr/bin/start.sh \"--help\"'\n" {
		t.Fatal("usr/bin/firebuild-entrypoint.sh did not contain required content")
	}
}

const testJsonData = `{
	"drives":{
	   "1":{
		  "drive-id":"1",
		  "is-read-only":"false",
		  "is-root-device":"true",
		  "partuuid":"",
		  "path-on-host":"rootfs"
	   }
	},
	"entrypoint-json": "{\"cmd\": [\"--help\"], \"entrypoint\": [\"/usr/bin/start.sh\"], \"env\": {\"ETCD_VERSION\": \"3.4.0\"}, \"shell\": [\"/bin/sh\", \"-c\"], \"user\": \"0:0\", \"workdir\": \"/\"}",
	"env":{
		"ENV_VAR": "a value"
	},
	"image-tag":"combust-labs/etcd:3.4.0",
	"local-hostname":"focused-edison",
	"machine":{
	   "cpu":"1",
	   "cpu-template":"",
	   "ht-enabled":"false",
	   "kernel-args":"console=ttyS0 noapic reboot=k panic=1 pci=off nomodules rw",
	   "mem":"128",
	   "vmlinux":"vmlinux-v5.8"
	},
	"network":{
	   "cni-network-name":"alpine",
	   "interfaces":{
		  "c6:15:a7:48:76:16":{
			 "gateway":"192.168.127.1",
			 "host-dev-name":"tap18",
			 "ifname":"",
			 "ip":"192.168.127.54",
			 "ip-addr":"192.168.127.54/24",
			 "ip-mask":"ffffff00",
			 "ip-net":"ip+net",
			 "nameservers":""
		  }
	   },
	   "ssh-port":"22"
	},
	"users":{
	   "alpine":{
		  "ssh-keys":"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDMY2vE7bgq4p4rCfiFfemkMu4P5pX7QA1qCDXu/3kzD/EO1S7jwBR69OTW5BCiOVgRfl+o2or5rBkDrsd6GKCJd3enqRLVqHazeWRJlRLx4W/uyM7n664SgFQ/Tno3g+NIo06XN8Ijhr0IGVsEF+FFO5rWOGVGANV5vuChd4QLtCGW6uJtNuNl6vCFcRU+wlYU/1QzfnuicTNGVQhsG1AIEhqmGRJYXWypOIE4s09z0T/rtD988678jINdPj3e5Gv5qBEra0IrgDTVncQfWW6m+T04uE88qYFzrgDR8rovljZiPKp3xFsBUK7Zkzkc5PIJJPaswnm4qYL2TuPVm1LnfjacrmZdaaIHepyiWNLZFClzwqz8lQqKLyXIccGELyGDibN8AEe2W7VbAoqNe9PGJSo4ooB5Owy97yyPE0VwTXwXiBZ/tjJu6U+/kDXzdhQFu+sJEoLmCOgh/+nZ1zLuP+qVJ7rWARX/GtsQYXN9ZcI+TnrqNQ33F8/l6J5SX/XSHX7wtHCpCa8JdyF4yRTz05UAGEezWPAXhjgckCkMriyaoEibBcNDMiUSB7ngXgs4EYHf5FyepWZw8UFceMLKrEbcPNRfQxnNmTCUU3F71NAHqEl//RESUnF5I4NgwxQnqBCe0sVhTAfLOfkddET88jpHjn5uOxFAelcPyWBW6Q==\n"
	   }
	},
	"vmm-id":"pkztxllhbaactacdyhea"
 }`
