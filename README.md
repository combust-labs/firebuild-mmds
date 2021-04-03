# firebuild MMDS metadata library

This library contains the MMDS metadata definition used by firebuild built machines and a program to load the VM configuration on the guest from the MMDS.

## manually querying the metadata from guest

```sh
curl -H 'Accept: application/json' http://169.254.169.254/latest/meta-data | jq '.'
```

Example of the output:

```json
{
   "Drives":{
      "1":{
         "DriveID":"1",
         "IsReadOnly":"false",
         "IsRootDevice":"true",
         "PartUUID":"",
         "PathOnHost":"rootfs"
      }
   },
   "Env":{},
   "ImageTag":"combust-labs/etcd:3.4.0",
   "LocalHostname":"focused-edison",
   "Machine":{
      "CPU":"1",
      "CPUTemplate":"",
      "HTEnabled":"false",
      "KernelArgs":"console=ttyS0 noapic reboot=k panic=1 pci=off nomodules rw",
      "Mem":"128",
      "VMLinux":"vmlinux-v5.8"
   },
   "Network":{
      "CniNetworkName":"alpine",
      "Interfaces":{
         "c6:15:a7:48:76:16":{
            "Gateway":"192.168.127.1",
            "HostDeviceName":"tap18",
            "IfName":"",
            "IP":"192.168.127.54",
            "IPAddr":"192.168.127.54/24",
            "IPMask":"ffffff00",
            "IPNet":"ip+net",
            "NameServers":""
         }
      }
   },
   "Users":{
      "alpine":{
         "SSHKeys":"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDMY2vE7bgq4p4rCfiFfemkMu4P5pX7QA1qCDXu/3kzD/EO1S7jwBR69OTW5BCiOVgRfl+o2or5rBkDrsd6GKCJd3enqRLVqHazeWRJlRLx4W/uyM7n664SgFQ/Tno3g+NIo06XN8Ijhr0IGVsEF+FFO5rWOGVGANV5vuChd4QLtCGW6uJtNuNl6vCFcRU+wlYU/1QzfnuicTNGVQhsG1AIEhqmGRJYXWypOIE4s09z0T/rtD988678jINdPj3e5Gv5qBEra0IrgDTVncQfWW6m+T04uE88qYFzrgDR8rovljZiPKp3xFsBUK7Zkzkc5PIJJPaswnm4qYL2TuPVm1LnfjacrmZdaaIHepyiWNLZFClzwqz8lQqKLyXIccGELyGDibN8AEe2W7VbAoqNe9PGJSo4ooB5Owy97yyPE0VwTXwXiBZ/tjJu6U+/kDXzdhQFu+sJEoLmCOgh/+nZ1zLuP+qVJ7rWARX/GtsQYXN9ZcI+TnrqNQ33F8/l6J5SX/XSHX7wtHCpCa8JdyF4yRTz05UAGEezWPAXhjgckCkMriyaoEibBcNDMiUSB7ngXgs4EYHf5FyepWZw8UFceMLKrEbcPNRfQxnNmTCUU3F71NAHqEl//RESUnF5I4NgwxQnqBCe0sVhTAfLOfkddET88jpHjn5uOxFAelcPyWBW6Q==\n"
      }
   },
   "VMMID":"pkztxllhbaactacdyhea"
}
```

## vminit

Build `vminit` for Linux:

```sh
make build-vminit
```

### run on the guest

Assuming that the guest VM is started with `--mmds` flag and MMDS defaults:

```sudo
sudo vminit
```

### functionality

`vminit` contacts the MMDS service from the gurst and downloads the MMDS data. After download, it does the following actions:

- if `latest/meta-data/Env` map contains any values, if writes the environment file in `/etc/profile.d/run-env.sh`
- if `latest/meta-data/LocalHostname` is not empty, writes the value to `/etc/hostname` file
- if rewrites `/etc/hosts` file to the defaults, additionally:
  - if `latest/meta-data/Network/Interfaces` contains interfaces and `latest/meta-data/LocalHostname` is not empty, adds an mapping entry for the interface IP address + hostname such that the VM can resolve its own hostname
- if `latest/meta-data/Users` contains user definitions, writes SSH authorized keys files for each respective user

## cutting releases

```sh
make release
```

This will create a release on GitHub.

```sh
make build-vmlinux
```

Upload the resulting binary artifact to the release.
