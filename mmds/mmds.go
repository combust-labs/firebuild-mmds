package mmds

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

var (
	defaultPingInterval = time.Second * 5
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
	Bootstrap      *MMDSBootstrap        `json:"Bootstrap,omitempty" mapstructure:"Bootstrap,omitempty"`
	VMMID          string                `json:"VMMID" mapstructure:"VMMID"`
	Drives         map[string]*MMDSDrive `json:"Drives" mapstructure:"Drives"`
	EntrypointJSON string                `json:"EntrypointJSON" mapstructure:"EntrypointJSON"`
	Env            map[string]string     `json:"Env" mapstructure:"Env"`
	LocalHostname  string                `json:"LocalHostname" mapstructure:"LocalHostname"`
	Machine        *MMDSMachine          `json:"Machine" mapstructure:"Machine"`
	Network        *MMDSNetwork          `json:"Network" mapstructure:"Network"`
	ImageTag       string                `json:"ImageTag" mapstructure:"ImageTag"`
	Users          map[string]*MMDSUser  `json:"Users" mapstructure:"Users"`
}

type MMDSBootstrap struct {
	HostPort     string `json:"HostPort" mapstructure:"HostPort"`
	CaChain      string `json:"CAChain" mapstructure:"CAChain"`
	Certificate  string `json:"Cert" mapstructure:"Cert"`
	Key          string `json:"Key" mapstructure:"Key"`
	ServerName   string `json:"ServerName" mapstructure:"ServerName"`
	PingInterval string `json:"PingInterval" mapstructure:"PingInterval"`
}

func (b *MMDSBootstrap) SafePingInterval() time.Duration {
	duration, err := time.ParseDuration(b.PingInterval)
	if err != nil {
		return defaultPingInterval
	}
	return duration
}

type MMDSDrive struct {
	DriveID      string `json:"DriveID" mapstructure:"DriveID"`
	IsReadOnly   string `json:"IsReadOnly" mapstructure:"IsReadOnly"`
	IsRootDevice string `json:"IsRootDevice" mapstructure:"IsRootDevice"`
	Partuuid     string `json:"PartUUID" mapstructure:"PartUUID"`
	PathOnHost   string `json:"PathOnHost" mapstructure:"PathOnHost"`
}

type MMDSNetwork struct {
	CNINetworkName string                           `json:"CniNetworkName" mapstructure:"CniNetworkName"`
	Interfaces     map[string]*MMDSNetworkInterface `json:"Interfaces" mapstructure:"Interfaces"`
}

type MMDSNetworkInterface struct {
	HostDeviceName string `json:"HostDeviceName" mapstructure:"HostDeviceName"`
	Gateway        string `json:"Gateway" mapstructure:"Gateway"`
	IfName         string `json:"IfName" mapstructure:"IfName"`
	IP             string `json:"IP" mapstructure:"IP"`
	IPAddr         string `json:"IPAddr" mapstructure:"IPAddr"`
	IPMask         string `json:"IPMask" mapstructure:"IPMask"`
	IPNet          string `json:"IPNet" mapstructure:"IPNet"`
	Nameservers    string `json:"NameServers" mapstructure:"NameServers"`
}

type MMDSUser struct {
	SSHKeys string `json:"SSHKeys" mapstructure:"SSHKeys"`
}

type MMDSMachine struct {
	CPU         string `json:"CPU" mapstructure:"CPU"`
	CPUTemplate string `json:"CPUTemplate" mapstructure:"CPUTemplate"`
	HTEnabled   string `json:"HTEnabled" mapstructure:"HTEnabled"`
	KernelArgs  string `json:"KernelArgs" mapstructure:"KernelArgs"`
	Mem         string `json:"Mem" mapstructure:"Mem"`
	VMLinuxID   string `json:"VMLinux" mapstructure:"VMLinux"`
}

type MMDSRootfsEntrypointInfo struct {
	Cmd        []string          `json:"Cmd" mapstructure:"Cmd"`
	Entrypoint []string          `json:"EntryPoint" mapstructure:"EntryPoint"`
	Env        map[string]string `json:"Env" mapstructure:"Env"`
	Shell      []string          `json:"Shell" mapstructure:"Shell"`
	User       string            `json:"User" mapstructure:"User"`
	Workdir    string            `json:"Workdir" mapstructure:"Workdir"`
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
func (inst *MMDSRootfsEntrypointInfo) ToShellCommand() (string, string, string) {
	// We're running the commands by wrapping the command in the shell call so sshSession.Setenv might not do what we intend.
	// Also, we don't really know which shell are we running because it comes as an argument to us
	// so we can't, for example, assume bourne shell -a...
	envString := ""
	for k, v := range inst.Env {
		envString = fmt.Sprintf("%sexport %s=\"%s\"; ", envString, k, v)
	}
	commandString := fmt.Sprintf("export PATH=$PATH:%s; cd %s && ", inst.Workdir, inst.Workdir)
	commandString = fmt.Sprintf("%s%s", commandString, strings.Join(inst.Entrypoint, " "))
	for _, c := range inst.Cmd {
		commandString = fmt.Sprintf("%s \"%s\"", commandString, c)
	}
	commandString = strings.ReplaceAll(commandString, "'", "'\\''")
	if len(inst.Shell) > 0 {
		return strings.Join(inst.Shell, " "), envString, commandString
	}
	return "/bin/sh -c", envString, commandString
}
