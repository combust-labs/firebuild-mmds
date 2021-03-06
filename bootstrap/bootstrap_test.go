package bootstrap

import (
	"bytes"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/combust-labs/firebuild-embedded-ca/ca"
	"github.com/combust-labs/firebuild-mmds/mmds"
	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/resources"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestFailingRunCommandBootstrap(t *testing.T) {

	testServerAppName := "test-server-app"

	logger := hclog.Default()
	logger.SetLevel(hclog.Trace)

	// recreate a work context manually:
	buildCtx := &rootfs.WorkContext{
		ExecutableCommands: []commands.VMInitSerializableCommand{
			commands.Run{
				OriginalCommand: "RUN echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Args: map[string]string{
					"BUILD_ARG": "value",
				},
				Command: "echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Env:     map[string]string{},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Run{
				OriginalCommand: "RUN exit 1",
				Args:            map[string]string{},
				Command:         "exit 1",
				Env:             map[string]string{},
				Shell:           commands.DefaultShell(),
				User:            commands.DefaultUser(),
				Workdir:         commands.DefaultWorkdir(),
			},
		},
	}

	// construct an embedded CA to manually handle TLS configs:
	embeddedCAConfig := &ca.EmbeddedCAConfig{
		Addresses:     []string{testServerAppName},
		CertsValidFor: time.Hour,
		KeySize:       1024,
	}

	embeddedCA, err := ca.NewDefaultEmbeddedCAWithLogger(embeddedCAConfig, logger.Named("embedded-ca"))
	if err != nil {
		t.Fatal("failed constructing embedded CA", err)
	}

	serverTLSConfig, err := embeddedCA.NewServerCertTLSConfig()
	if err != nil {
		t.Fatal("failed creating test server TLS config", err)
	}

	grpcConfig := &rootfs.GRPCServiceConfig{
		ServerName:      testServerAppName,
		BindHostPort:    "127.0.0.1:0",
		TLSConfigServer: serverTLSConfig,
	}

	testServer := rootfs.NewTestServer(t, logger.Named("grpc-server"), grpcConfig, buildCtx)
	testServer.Start()
	select {
	case startErr := <-testServer.FailedNotify():
		t.Fatal("expected the GRPC server to start but it failed", startErr)
	case <-testServer.ReadyNotify():
		t.Log("GRPC server started and serving on", grpcConfig.BindHostPort)
	}

	clientCertData, err := embeddedCA.NewClientCert()
	if err != nil {
		t.Fatal("failed creating test client certitifcate", err)
	}

	bootstrapConfig := &mmds.MMDSBootstrap{
		HostPort:    grpcConfig.BindHostPort,
		CaChain:     strings.Join(embeddedCA.CAPEMChain(), "\n"),
		Certificate: string(clientCertData.CertificatePEM()),
		Key:         string(clientCertData.KeyPEM()),
		ServerName:  testServerAppName,
	}

	bootstrapper := NewDefaultBoostrapper(logger.Named("bootstrapper"), bootstrapConfig).
		WithCommandRunner(NewShellCommandRunner(logger.Named("shell-runner"))).
		WithResourceDeployer(NewExecutingResourceDeployer(logger.Named("executing-deployer")))

	bootstrapErr := bootstrapper.Execute()
	assert.NotNil(t, bootstrapErr)

	<-testServer.FinishedNotify()

	serverOutput := testServer.ReceivedStdout()
	assert.Equal(t, len(serverOutput), 1)
}

func TestFailingAddBootstrap(t *testing.T) {

	testServerAppName := "test-server-app"

	logger := hclog.Default()
	logger.SetLevel(hclog.Debug)

	// use this directory as the workdir for ADD and COPY resources:
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp dir, got error", err)
	}
	defer os.RemoveAll(tempDir)

	// recreate a work context manually:
	buildCtx := &rootfs.WorkContext{
		ExecutableCommands: []commands.VMInitSerializableCommand{
			commands.Run{
				OriginalCommand: "RUN apt-get update && apt-get install ca-certificates && mkdir -p ${HOME}/test",
				Args:            map[string]string{},
				Command:         "apt-get update && apt-get install ca-certificates && mkdir -p ${HOME}/test",
				Env: map[string]string{
					"HOME": "/home/test-user",
				},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Run{
				OriginalCommand: "RUN echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Args: map[string]string{
					"BUILD_ARG": "value",
				},
				Command: "echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Env:     map[string]string{},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Add{
				OriginalCommand: "ADD etc/test-file1 /etc/test-file1",
				OriginalSource:  "etc/test-file1",
				Source:          "etc/test-file1",
				Target:          "/etc/test-file1",
				User:            commands.DefaultUser(),
				Workdir:         commands.Workdir{Value: tempDir},
			},
		},
	}

	// construct an embedded CA to manually handle TLS configs:
	embeddedCAConfig := &ca.EmbeddedCAConfig{
		Addresses:     []string{testServerAppName},
		CertsValidFor: time.Hour,
		KeySize:       1024,
	}

	embeddedCA, err := ca.NewDefaultEmbeddedCAWithLogger(embeddedCAConfig, logger.Named("embedded-ca"))
	if err != nil {
		t.Fatal("failed constructing embedded CA", err)
	}

	serverTLSConfig, err := embeddedCA.NewServerCertTLSConfig()
	if err != nil {
		t.Fatal("failed creating test server TLS config", err)
	}

	grpcConfig := &rootfs.GRPCServiceConfig{
		ServerName:      testServerAppName,
		BindHostPort:    "127.0.0.1:0",
		TLSConfigServer: serverTLSConfig,
	}

	testServer := rootfs.NewTestServer(t, logger.Named("grpc-server"), grpcConfig, buildCtx)
	testServer.Start()
	select {
	case startErr := <-testServer.FailedNotify():
		t.Fatal("expected the GRPC server to start but it failed", startErr)
	case <-testServer.ReadyNotify():
		t.Log("GRPC server started and serving on", grpcConfig.BindHostPort)
	}

	clientCertData, err := embeddedCA.NewClientCert()
	if err != nil {
		t.Fatal("failed creating test client certitifcate", err)
	}

	bootstrapConfig := &mmds.MMDSBootstrap{
		HostPort:    grpcConfig.BindHostPort,
		CaChain:     strings.Join(embeddedCA.CAPEMChain(), "\n"),
		Certificate: string(clientCertData.CertificatePEM()),
		Key:         string(clientCertData.KeyPEM()),
		ServerName:  testServerAppName,
	}

	bootstrapper := NewDefaultBoostrapper(logger.Named("bootstrapper"), bootstrapConfig).
		WithCommandRunner(NewShellCommandRunner(logger.Named("shell-runner"))).
		WithResourceDeployer(NewExecutingResourceDeployer(logger.Named("executing-deployer")))

	bootstrapErr := bootstrapper.Execute()
	assert.NotNil(t, bootstrapErr)

	<-testServer.FinishedNotify()

	serverOutput := testServer.ReceivedStdout()
	assert.Equal(t, len(serverOutput), 2)
}

func TestFailingCopyBootstrap(t *testing.T) {

	testServerAppName := "test-server-app"

	logger := hclog.Default()
	logger.SetLevel(hclog.Debug)

	// use this directory as the workdir for ADD and COPY resources:
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp dir, got error", err)
	}
	defer os.RemoveAll(tempDir)

	// recreate a work context manually:
	buildCtx := &rootfs.WorkContext{
		ExecutableCommands: []commands.VMInitSerializableCommand{
			commands.Run{
				OriginalCommand: "RUN apt-get update && apt-get install ca-certificates && mkdir -p ${HOME}/test",
				Args:            map[string]string{},
				Command:         "apt-get update && apt-get install ca-certificates && mkdir -p ${HOME}/test",
				Env: map[string]string{
					"HOME": "/home/test-user",
				},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Run{
				OriginalCommand: "RUN echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Args: map[string]string{
					"BUILD_ARG": "value",
				},
				Command: "echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Env:     map[string]string{},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Copy{
				OriginalCommand: "COPY etc/directory /etc/directory",
				OriginalSource:  "etc/directory",
				Source:          "etc/directory",
				Target:          "/etc/directory",
				User:            commands.DefaultUser(),
				Workdir:         commands.Workdir{Value: tempDir},
			},
		},
	}

	// construct an embedded CA to manually handle TLS configs:
	embeddedCAConfig := &ca.EmbeddedCAConfig{
		Addresses:     []string{testServerAppName},
		CertsValidFor: time.Hour,
		KeySize:       1024,
	}

	embeddedCA, err := ca.NewDefaultEmbeddedCAWithLogger(embeddedCAConfig, logger.Named("embedded-ca"))
	if err != nil {
		t.Fatal("failed constructing embedded CA", err)
	}

	serverTLSConfig, err := embeddedCA.NewServerCertTLSConfig()
	if err != nil {
		t.Fatal("failed creating test server TLS config", err)
	}

	grpcConfig := &rootfs.GRPCServiceConfig{
		ServerName:      testServerAppName,
		BindHostPort:    "127.0.0.1:0",
		TLSConfigServer: serverTLSConfig,
	}

	testServer := rootfs.NewTestServer(t, logger.Named("grpc-server"), grpcConfig, buildCtx)
	testServer.Start()
	select {
	case startErr := <-testServer.FailedNotify():
		t.Fatal("expected the GRPC server to start but it failed", startErr)
	case <-testServer.ReadyNotify():
		t.Log("GRPC server started and serving on", grpcConfig.BindHostPort)
	}

	clientCertData, err := embeddedCA.NewClientCert()
	if err != nil {
		t.Fatal("failed creating test client certitifcate", err)
	}

	bootstrapConfig := &mmds.MMDSBootstrap{
		HostPort:    grpcConfig.BindHostPort,
		CaChain:     strings.Join(embeddedCA.CAPEMChain(), "\n"),
		Certificate: string(clientCertData.CertificatePEM()),
		Key:         string(clientCertData.KeyPEM()),
		ServerName:  testServerAppName,
	}

	bootstrapper := NewDefaultBoostrapper(logger.Named("bootstrapper"), bootstrapConfig).
		WithCommandRunner(NewShellCommandRunner(logger.Named("shell-runner"))).
		WithResourceDeployer(NewExecutingResourceDeployer(logger.Named("executing-deployer")))

	bootstrapErr := bootstrapper.Execute()
	assert.NotNil(t, bootstrapErr)

	<-testServer.FinishedNotify()

	serverOutput := testServer.ReceivedStdout()
	assert.Equal(t, len(serverOutput), 2)
}

func TestSuccessfulBootstrapWithResources(t *testing.T) {

	testServerAppName := "test-server-app"

	logger := hclog.Default()
	logger.SetLevel(hclog.Debug)

	// use this directory as the workdir for ADD and COPY resources:
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("expected temp dir, got error", err)
	}
	defer os.RemoveAll(tempDir)

	etcTestFile1Contents := []byte("test-file1 contents")

	rootfs.MustPutTestResource(t, filepath.Join(tempDir, "etc/test-file1"), etcTestFile1Contents)
	rootfs.MustPutTestResource(t, filepath.Join(tempDir, "etc/directory/file1"), []byte("etc/directory/file1 contents"))
	rootfs.MustPutTestResource(t, filepath.Join(tempDir, "etc/directory/file2"), []byte("etc/directory/file2 contents"))
	rootfs.MustPutTestResource(t, filepath.Join(tempDir, "etc/directory/subdir/subdir-file1"), []byte("etc/directory/subdir/subdir-file1 contents"))

	rootfs.MustPutTestResource(t, filepath.Join(tempDir, "relative-file"), []byte("etc/directory/subdir/relative-file contents"))

	// recreate a work context manually:
	buildCtx := &rootfs.WorkContext{
		ExecutableCommands: []commands.VMInitSerializableCommand{
			commands.Run{
				OriginalCommand: "RUN apt-get update && apt-get install ca-certificates && mkdir -p ${HOME}/test",
				Args:            map[string]string{},
				Command:         "apt-get update && apt-get install ca-certificates && mkdir -p ${HOME}/test",
				Env: map[string]string{
					"HOME": "/home/test-user",
				},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Run{
				OriginalCommand: "RUN echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Args: map[string]string{
					"BUILD_ARG": "value",
				},
				Command: "echo ${BUILD_ARG}; apkArch=\"$(apk --print-arch)\" && case \"${apkArch}\"",
				Env:     map[string]string{},
				Shell: commands.Shell{
					Commands: []string{"/bin/echo", "-e"},
				},
				User:    commands.DefaultUser(),
				Workdir: commands.DefaultWorkdir(),
			},
			commands.Add{
				OriginalCommand: "ADD etc/test-file1 /etc/test-file1",
				OriginalSource:  "etc/test-file1",
				Source:          "etc/test-file1",
				Target:          "/etc/test-file1",
				User:            commands.DefaultUser(),
				Workdir:         commands.Workdir{Value: tempDir},
			},
			commands.Copy{
				OriginalCommand: "COPY etc/directory /etc/directory",
				OriginalSource:  "etc/directory",
				Source:          "etc/directory",
				Target:          "/etc/directory",
				User:            commands.DefaultUser(),
				Workdir:         commands.Workdir{Value: tempDir},
			},
			commands.Copy{
				OriginalCommand: "COPY relative-file /etc/directory/subdir/relative-file",
				OriginalSource:  "relative-file",
				Source:          "relative-file",
				Target:          "/etc/directory/subdir/relative-file",
				User:            commands.DefaultUser(),
				Workdir:         commands.Workdir{Value: tempDir},
			},
		},
		ResourcesResolved: rootfs.Resources{
			"etc/test-file1": []resources.ResolvedResource{
				resources.NewResolvedFileResourceWithPath(func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(etcTestFile1Contents)), nil
				},
					fs.FileMode(0755),
					"etc/test-file1",
					"/etc/test-file1",
					commands.Workdir{Value: tempDir},
					commands.DefaultUser(),
					filepath.Join(tempDir, "etc/test-file1")),
			},
			"etc/directory": []resources.ResolvedResource{
				resources.NewResolvedDirectoryResourceWithPath(fs.FileMode(0755),
					filepath.Join(tempDir, "etc/directory"),
					"etc/directory",
					"/etc/directory",
					commands.Workdir{Value: tempDir},
					commands.DefaultUser()),
			},
			"relative-file": []resources.ResolvedResource{
				resources.NewResolvedFileResourceWithPath(func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(etcTestFile1Contents)), nil
				},
					fs.FileMode(0755),
					"relative-file",
					"/etc/directory/subdir/relative-file",
					commands.Workdir{Value: tempDir},
					commands.DefaultUser(),
					filepath.Join(tempDir, "relative-file")),
			},
		},
	}

	// construct an embedded CA to manually handle TLS configs:
	embeddedCAConfig := &ca.EmbeddedCAConfig{
		Addresses:     []string{testServerAppName},
		CertsValidFor: time.Hour,
		KeySize:       1024,
	}

	embeddedCA, err := ca.NewDefaultEmbeddedCAWithLogger(embeddedCAConfig, logger.Named("embedded-ca"))
	if err != nil {
		t.Fatal("failed constructing embedded CA", err)
	}

	serverTLSConfig, err := embeddedCA.NewServerCertTLSConfig()
	if err != nil {
		t.Fatal("failed creating test server TLS config", err)
	}

	grpcConfig := &rootfs.GRPCServiceConfig{
		ServerName:      testServerAppName,
		BindHostPort:    "127.0.0.1:0",
		TLSConfigServer: serverTLSConfig,
	}

	testServer := rootfs.NewTestServer(t, logger.Named("grpc-server"), grpcConfig, buildCtx)
	testServer.Start()
	select {
	case startErr := <-testServer.FailedNotify():
		t.Fatal("expected the GRPC server to start but it failed", startErr)
	case <-testServer.ReadyNotify():
		t.Log("GRPC server started and serving on", grpcConfig.BindHostPort)
	}

	clientCertData, err := embeddedCA.NewClientCert()
	if err != nil {
		t.Fatal("failed creating test client certitifcate", err)
	}

	bootstrapConfig := &mmds.MMDSBootstrap{
		HostPort:    grpcConfig.BindHostPort,
		CaChain:     strings.Join(embeddedCA.CAPEMChain(), "\n"),
		Certificate: string(clientCertData.CertificatePEM()),
		Key:         string(clientCertData.KeyPEM()),
		ServerName:  testServerAppName,
	}

	bootstrapper := NewDefaultBoostrapper(logger.Named("bootstrapper"), bootstrapConfig).
		WithCommandRunner(NewShellCommandRunner(logger.Named("shell-runner"))).
		WithResourceDeployer(NewExecutingResourceDeployer(logger.Named("executing-deployer")))

	bootstrapErr := bootstrapper.Execute()
	assert.Nil(t, bootstrapErr)

	<-testServer.FinishedNotify()

	serverOutput := testServer.ReceivedStdout()
	assert.Equal(t, len(serverOutput), 2)
}

func TestGetTLSConfig(t *testing.T) {

	logger := hclog.Default()
	logger.SetLevel(hclog.Debug)

	embeddedCAConfig := &ca.EmbeddedCAConfig{
		Addresses:     []string{"test-app"},
		CertsValidFor: time.Hour,
		KeySize:       1024,
	}

	embeddedCA, err := ca.NewDefaultEmbeddedCAWithLogger(embeddedCAConfig, logger.Named("embedded-ca"))
	if err != nil {
		t.Fatal("failed constructing embedded CA", err)
	}

	clientCertData, err := embeddedCA.NewClientCert()
	if err != nil {
		t.Fatal("failed creating test client certitifcate", err)
	}

	bootstrapConfig := &mmds.MMDSBootstrap{
		HostPort:    "127.0.0.1:0",
		CaChain:     strings.Join(embeddedCA.CAPEMChain(), "\n"),
		Certificate: string(clientCertData.CertificatePEM()),
		Key:         string(clientCertData.KeyPEM()),
		ServerName:  "irrelevant",
	}

	_, tlsConfigErr := getTLSConfig(bootstrapConfig)
	if tlsConfigErr != nil {
		t.Fatal("expected TLS config, got error", tlsConfigErr)
	}

}

const testDockerfileMultiStage = `FROM alpine:3.13 as builder

FROM alpine:3.13
ARG PARAM1=value
ENV ENVPARAM1=envparam1
RUN mkdir -p /dir
ADD resource1 /target/resource1
COPY resource2 /target/resource2
COPY --from=builder /etc/test /etc/test
RUN cp /dir/${ENVPARAM1} \
	&& call --arg=${PARAM1}`
