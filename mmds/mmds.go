package mmds

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

type MMDSLatest struct {
	Latest *MMDSLatestMetadata `json:"latest" mapstructure:"latest"`
}

func (r *MMDSLatest) Serialize() (interface{}, error) {
	output := map[string]interface{}{}
	if err := mapstructure.Decode(r, &output); err != nil {
		return nil, err
	}
	return output, nil
}

type MMDSLatestMetadata struct {
	Metadata *MMDSData `json:"meta-data" mapstructure:"meta-data"`
}

type MMDSData struct {
	VMMID          string                `json:"vmm-id" mapstructure:"vmm-id"`
	Drives         map[string]*MMDSDrive `json:"drives" mapstructure:"drives"`
	EntrypointJSON string                `json:"entrypoint-json" mapstructure:"entrypoint-json"`
	Env            map[string]string     `json:"env" mapstructure:"env"`
	LocalHostname  string                `json:"local-hostname" mapstructure:"local-hostname"`
	Machine        *MMDSMachine          `json:"machine" mapstructure:"machine"`
	Network        *MMDSNetwork          `json:"network" mapstructure:"network"`
	ImageTag       string                `json:"image-tag" mapstructure:"image-tag"`
	Users          map[string]*MMDSUser  `json:"users" mapstructure:"users"`
}

type MMDSDrive struct {
	DriveID      string `json:"drive-id" mapstructure:"drive-id"`
	IsReadOnly   string `json:"is-read-only" mapstructure:"is-read-only"`
	IsRootDevice string `json:"is-root-device" mapstructure:"is-root-device"`
	Partuuid     string `json:"partuuid" mapstructure:"partuuid"`
	PathOnHost   string `json:"path-on-host" mapstructure:"path-on-host"`
}

type MMDSNetwork struct {
	CNINetworkName string                           `json:"cni-network-name" mapstructure:"cni-network-name"`
	Interfaces     map[string]*MMDSNetworkInterface `json:"interfaces" mapstructure:"interfaces"`
	SSHPort        string                           `json:"ssh-port" mapstructure:"ssh-port"`
}

type MMDSNetworkInterface struct {
	HostDevName string `json:"host-dev-name" mapstructure:"host-dev-name"`
	Gateway     string `json:"gateway" mapstructure:"gateway"`
	IfName      string `json:"ifname" mapstructure:"ifname"`
	IP          string `json:"ip" mapstructure:"ip"`
	IPAddr      string `json:"ip-addr" mapstructure:"ip-addr"`
	IPMask      string `json:"ip-mask" mapstructure:"ip-mask"`
	IPNet       string `json:"ip-net" mapstructure:"ip-net"`
	Nameservers string `json:"nameservers" mapstructure:"nameservers"`
}

type MMDSUser struct {
	SSHKeys string `json:"ssh-keys" mapstructure:"ssh-keys"`
}

type MMDSMachine struct {
	CPU         string `json:"cpu" mapstructure:"cpu"`
	CPUTemplate string `json:"cpu-template" mapstructure:"cpu-template"`
	HTEnabled   string `json:"ht-enabled" mapstructure:"ht-enabled"`
	KernelArgs  string `json:"kernel-args" mapstructure:"kernel-args"`
	Mem         string `json:"mem" mapstructure:"mem"`
	VMLinuxID   string `json:"vmlinux" mapstructure:"vmlinux"`
}

type MMDSRootfsEntrypointInfo struct {
	Cmd        []string          `json:"cmd" mapstructure:"cmd"`
	Entrypoint []string          `json:"entrypoint" mapstructure:"entrypoint"`
	Env        map[string]string `json:"env" mapstructure:"env"`
	Shell      []string          `json:"shell" mapstructure:"shell"`
	User       string            `json:"user" mapstructure:"user"`
	Workdir    string            `json:"workdir" mapstructure:"workdir"`
}

// NewMMDSRootfsEntrypointInfoFromJSON deserializes a JSON string to a *MMDSRootfsEntrypointInfo.
func NewMMDSRootfsEntrypointInfoFromJSON(input string) (*MMDSRootfsEntrypointInfo, error) {
	output := &MMDSRootfsEntrypointInfo{}
	return output, json.Unmarshal([]byte(input), output)
}

// ToJsonString converts the rootfs entrypoint info to a JSON string.
func (inst *MMDSRootfsEntrypointInfo) ToJsonString() (string, error) {
	bytes, err := json.Marshal(inst)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToShellCommand returns two strings representing a shell in which the command must be executed and a command itself.
// The final execution of the command should be done in the following way:
//   shell 'actual-command'
func (inst *MMDSRootfsEntrypointInfo) ToShellCommand() (string, string) {
	// We're running the commands by wrapping the command in the shell call so sshSession.Setenv might not do what we intend.
	// Also, we don't really know which shell are we running because it comes as an argument to us
	// so we can't, for example, assume bourne shell -a...
	envString := ""
	for k, v := range inst.Env {
		envString = fmt.Sprintf("%s%s=\"%s\"; ", envString, k, v)
	}
	commandString := fmt.Sprintf("%scd %s && ", envString, inst.Workdir)
	commandString = fmt.Sprintf("%s%s ", commandString, strings.Join(inst.Entrypoint, " "))
	for _, c := range inst.Cmd {
		commandString = fmt.Sprintf("%s\"%s\"", commandString, c)
	}
	commandString = strings.ReplaceAll(commandString, "'", "'\\''")
	if len(inst.Shell) > 0 {
		return strings.Join(inst.Shell, " "), commandString
	}
	return "/bin/sh -c", commandString
}
