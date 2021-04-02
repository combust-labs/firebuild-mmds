package bootstrap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/combust-labs/firebuild-shared/build/commands"
	"github.com/combust-labs/firebuild-shared/build/resources"
	"github.com/combust-labs/firebuild-shared/build/rootfs"
	"github.com/hashicorp/go-hclog"
)

type ResourceDeployer interface {
	Add(commands.Add, rootfs.ClientProvider) error
	Copy(commands.Copy, rootfs.ClientProvider) error
}

type noopResourceDeployer struct {
	logger hclog.Logger
}

func (n *noopResourceDeployer) Add(cmd commands.Add, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing ADD command", "command", cmd)
	return nil
}
func (n *noopResourceDeployer) Copy(cmd commands.Copy, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing COPY command", "command", cmd)
	return nil
}

type executingResourceDeployer struct {
	defaultUser commands.User
	logger      hclog.Logger
}

func NewExecutingResourceDeployer(logger hclog.Logger) ResourceDeployer {
	return &executingResourceDeployer{
		defaultUser: commands.DefaultUser(),
		logger:      logger,
	}
}

func (n *executingResourceDeployer) Add(cmd commands.Add, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing ADD command", "command", cmd)
	return n.deployResources(cmd.Source, grpcClient)
}
func (n *executingResourceDeployer) Copy(cmd commands.Copy, grpcClient rootfs.ClientProvider) error {
	n.logger.Debug("executing COPY command", "command", cmd)
	return n.deployResources(cmd.Source, grpcClient)
}

func (n *executingResourceDeployer) deployResources(source string, grpcClient rootfs.ClientProvider) error {

	resourceChannel, err := grpcClient.Resource(source)

	if err != nil {
		return err
	}

	nResourcesTransferred := 0

	for {
		select {
		case item := <-resourceChannel:
			switch titem := item.(type) {
			case nil:
				if nResourcesTransferred == 0 {
					// there was nothing transferred, this is an error implying the resource was not found:
					n.logger.Error("no resources transferred for",
						"resource-path", source)
					return os.ErrNotExist
				}
				n.logger.Debug("resource deployed",
					"resource-path", source,
					"number-of-resources", nResourcesTransferred)
				return nil // finished successfully
			case resources.ResolvedResource:

				nResourcesTransferred = nResourcesTransferred + 1

				fullTargetResourcePath := filepath.Join(titem.TargetWorkdir().Value, titem.TargetPath())

				if titem.IsDir() {

					// create a directory:
					if err := os.MkdirAll(fullTargetResourcePath, titem.TargetMode()); err != nil {
						n.logger.Error("error while creating directory",
							"resource-path", titem.TargetPath(),
							"on-disk-path", fullTargetResourcePath)
						return err
					}

					n.logger.Debug("created directory",
						"resource-path", titem.TargetPath(),
						"on-disk-path", fullTargetResourcePath)

					if titem.TargetUser().Value != n.defaultUser.Value {
						uid, gid, err := stringToUidAndGid(titem.TargetUser().Value)
						if err != nil {
							n.logger.Error("error while chowning directory",
								"resource-path", titem.TargetPath(),
								"on-disk-path", fullTargetResourcePath,
								"reason", err)
							return err
						}
						if err := os.Chown(fullTargetResourcePath, uid, gid); err != nil {
							n.logger.Error("error while chowning directory",
								"resource-path", titem.TargetPath(),
								"on-disk-path", fullTargetResourcePath,
								"reason", err)
							return err
						}
					}
					continue
				}

				resourceReader, err := titem.Contents()
				if err != nil {
					n.logger.Error("error while fetching resource reader",
						"resource-path", titem.TargetPath(),
						"on-disk-path", fullTargetResourcePath,
						"reason", err)
					return err
				}
				defer resourceReader.Close()

				targetFile, err := os.OpenFile(fullTargetResourcePath, os.O_CREATE|os.O_RDWR, titem.TargetMode())

				if err != nil {
					n.logger.Error("error while creating target file",
						"resource-path", titem.TargetPath(),
						"on-disk-path", fullTargetResourcePath,
						"reason", err)
					return err
				}

				written, err := io.Copy(targetFile, resourceReader)
				if err != nil {
					targetFile.Close()
					n.logger.Error("error while writing target file",
						"resource-path", titem.TargetPath(),
						"on-disk-path", fullTargetResourcePath,
						"reason", err)
					return err
				}

				targetFile.Close()

				n.logger.Info("file written",
					"resource-path", titem.TargetPath(),
					"on-disk-path", fullTargetResourcePath,
					"written-bytes", written)

				// chown the file:

				if titem.TargetUser().Value != n.defaultUser.Value {
					uid, gid, err := stringToUidAndGid(titem.TargetUser().Value)
					if err != nil {
						n.logger.Error("error while chowning file",
							"resource-path", titem.TargetPath(),
							"on-disk-path", fullTargetResourcePath,
							"reason", err)
						return err
					}
					if err := os.Chown(fullTargetResourcePath, uid, gid); err != nil {
						n.logger.Error("error while chowning file",
							"resource-path", titem.TargetPath(),
							"on-disk-path", fullTargetResourcePath,
							"reason", err)
						return err
					}
				}

			case error:
				return titem
			}
		}
	}

}

func stringToUidAndGid(input string) (int, int, error) {
	parts := strings.Split(input, ":")
	if len(parts) == 0 {
		return -1, -1, fmt.Errorf("empty uid:gid")
	}
	if len(parts) == 1 {
		// uid only:
		uid, err := strconv.Atoi(parts[0])
		return uid, -1, err
	}
	if len(parts) == 2 {
		uid, uiderr := strconv.Atoi(parts[0])
		if uiderr != nil {
			return uid, -1, uiderr
		}
		gid, giderr := strconv.Atoi(parts[1])
		return uid, gid, giderr
	}
	return -1, -1, fmt.Errorf("invalid uid:gid")
}
