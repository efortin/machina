package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Code-Hex/vz"
	"github.com/efortin/machina/utils"
	"github.com/mitchellh/go-ps"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)
import "C"

const (
	default_disk_size = 15 * 1024 * 1024 * 1024
	default_mem_size  = 2 * 1024 * 1024 * 1024
	max_mem_size      = 8 * 1024 * 1024 * 1024
	pidFileName       = "vmz.pid"
	infoFileName      = "spec.json"

	commandPrefix = "machina"

	Machine_state_running = 0
	Machine_state_stop    = 1
	Machine_state_error   = 2
)

type MachineSpec struct {
	Cpu uint   `json:"cpu"`
	Ram uint64 `json:"memory"`
}

type Machine struct {
	Name         string              `json:"name"`
	Distribution *UbuntuDistribution `json:"distribution"`
	Spec         MachineSpec         `json:"specs"`
}

func (d *Machine) PidFilePath() string {
	return fmt.Sprintf("%s/%s", d.BaseDirectory(), pidFileName)
}

func InfoFilePath(machineName string) string {
	return fmt.Sprintf("%s/%s", MachineDirectory(machineName), infoFileName)
}

func (d *Machine) InfoFilePath() string {
	return InfoFilePath(d.Name)
}

/*
* Returns a ps.Process instance if it could find a vfkit process with the pid
* stored in $pidFileName
*
* Returns nil, nil if:
* - if the $pidFileName file does not exist,
* - if a process with the pid from this file cannot be found,
* - if a process was found, but its name is not 'vfkit'
 */
func (d *Machine) FindVfkitProcess() (ps.Process, int, error) {
	pidFile := d.PidFilePath()
	pid, err := readPidFromFile(pidFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, Machine_state_stop, nil
		}
		return nil, Machine_state_stop, fmt.Errorf("%v error reading pidfile %s", err, pidFile)
	}

	p, err := ps.FindProcess(pid)
	if err != nil {
		return nil, Machine_state_error, fmt.Errorf(fmt.Sprintf("%v cannot find pid %d", err, pid))
	}
	if p == nil {
		utils.Logger.Infof("vfkit pid %d missing from process table", pid)
		// return PidNotExist error?
		return nil, Machine_state_error, nil
	}

	if !strings.Contains(p.Executable(), commandPrefix) {
		// return InvalidExecutable error?
		utils.Logger.Infof("pid %d is stale, and is being used by %s", pid, p.Executable())
		return nil, Machine_state_error, nil
	}

	return p, Machine_state_running, nil
}

// Stop stops a host forcefully
func (d *Machine) Stop() {
	d.sendSignal(syscall.SIGTERM)

}

func (m *Machine) sendSignal(s os.Signal) error {
	psProc, machinestate, err := m.FindVfkitProcess()
	if machinestate != Machine_state_running {
		os.Remove(m.PidFilePath())
		return nil
	}
	utils.Logger.Info("try to kill", psProc.Pid())
	proc, err := os.FindProcess(psProc.Pid())
	if err == nil {
		return proc.Signal(s)
	} else {
		utils.Logger.Info("Error during kill", err)
	}
	return nil
}

// IpAddress Return VM ip address if already available
// error if not found
func (m *Machine) IpAddress() (string, error) {
	return GetIPAddressByMACAddress(GenerateAlmostUniqueMac(m.Name))
}

func (m *Machine) OutputFilePath() string {
	return fmt.Sprintf("%s/%s", m.BaseDirectory(), "output")
}

func (m *Machine) Output() *os.File {
	outputFile, _ := os.Create(m.OutputFilePath())
	return outputFile
}

func (m *Machine) inputFilePath() string {
	return fmt.Sprintf("%s/%s", m.BaseDirectory(), "input")
}

func (m *Machine) Input() *os.File {
	file, err := os.Create(m.inputFilePath())
	if err != nil {
		utils.Logger.Fatal("Cannot create input for for", m.Name, "at ", m.inputFilePath(), "with the following error", err.Error())
	}
	return file
}

func MachineDirectory(machineName string) string {
	return fmt.Sprintf("%s/%s", baseMachineDirectory(), machineName)
}

func (m *Machine) BaseDirectory() string {
	basedir := MachineDirectory(m.Name)

	if _, err := os.Stat(basedir); errors.Is(err, os.ErrNotExist) {
		log.Default().Println("Machine directory not found, creating it...")
		if err := os.Mkdir(basedir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
	return basedir
}

func (m *Machine) InitRdDirectory() (path string) {
	path = fmt.Sprintf("%s/%s", m.BaseDirectory(), "initrd")
	err := m.Distribution.cloneIfNotExist(m.Distribution.InitRdPath(), path)
	if err != nil {
		utils.Logger.Fatal(err)
	}
	return
}

func (m *Machine) KernelDirectory() (path string) {
	path = fmt.Sprintf("%s/%s", m.BaseDirectory(), "vmlinuz")
	err := m.Distribution.cloneIfNotExist(m.Distribution.KernelPath(), path)
	if err != nil {
		utils.Logger.Fatal(err)
	}
	return
}

func (m *Machine) RootDirectory() (path string, err error) {
	path = fmt.Sprintf("%s/%s", m.BaseDirectory(), "root.img")
	err = m.Distribution.cloneIfNotExist(m.Distribution.ImagePath(), path)
	if err != nil {
		return
	}
	disk, err := os.Stat(path)

	if default_disk_size > disk.Size() {
		utils.Logger.Info("Resizing disk", disk.Size(), "to", default_disk_size)
		os.Truncate(path, default_disk_size)
	}
	return
}

func (m *Machine) ExportMachineSpecification() {
	specContent, _ := json.MarshalIndent(m, "", "\t")
	_ = ioutil.WriteFile(m.InfoFilePath(), specContent, 0644)
}

func (m *Machine) hasAlreadyBeenConfigured() bool {
	// Find something else like check config or ip
	//_, err := os.Stat(m.InfoFilePath())
	return false
}

func (m *Machine) Run() {
	var err error
	switch m.State() {
	case Machine_state_running:
		utils.Logger.Info("Machine", m.Name, "has already been start by another process...")
		os.Exit(1)
	default:

		if !m.hasAlreadyBeenConfigured() {
			_, err = m.launchPrimaryBoot()
		}
		if err == nil {
			m.launch()
		}
	}
}

func (m *Machine) State() int {
	_, state, _ := m.FindVfkitProcess()
	return state
}

func (m *Machine) launch() {

	kernelCommandLineArguments := []string{"console=hvc0", "root=/dev/vda"}

	bootLoader := vz.NewLinuxBootLoader(
		m.KernelDirectory(),
		vz.WithCommandLine(strings.Join(kernelCommandLineArguments, " ")),
		vz.WithInitrd(m.InitRdDirectory()),
	)

	config := vz.NewVirtualMachineConfiguration(
		bootLoader,
		m.Spec.Cpu,
		m.Spec.Ram,
	)

	// console
	input, _ := os.Create(m.inputFilePath())
	serialPortAttachment := vz.NewFileHandleSerialPortAttachment(input, m.Output())
	consoleConfig := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
		consoleConfig,
	})

	// network
	natAttachment := vz.NewNATNetworkDeviceAttachment()
	networkConfig := vz.NewVirtioNetworkDeviceConfiguration(natAttachment)

	mac, err := net.ParseMAC(GenerateAlmostUniqueMac(m.Name))
	networkConfig.SetMacAddress(vz.NewMACAddress(mac))
	config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{
		networkConfig,
	})

	// entropy
	entropyConfig := vz.NewVirtioEntropyDeviceConfiguration()
	config.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{
		entropyConfig,
	})

	diskPath, err := m.RootDirectory()
	if err != nil {
		return
	}
	diskImageAttachment, err := vz.NewDiskImageStorageDeviceAttachment(
		diskPath,
		false,
	)

	if err != nil {
		log.Fatal(err)
	}
	storageDeviceConfig := vz.NewVirtioBlockDeviceConfiguration(diskImageAttachment)
	config.SetStorageDevicesVirtualMachineConfiguration([]vz.StorageDeviceConfiguration{
		storageDeviceConfig,
	})

	// traditional memory balloon device which allows for managing guest memory. (optional)
	config.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{
		vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration(),
	})

	// socket device (optional)
	config.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{
		vz.NewVirtioSocketDeviceConfiguration(),
	})
	validated, err := config.Validate()
	if !validated || err != nil {
		log.Fatal("validation failed", err)
	}

	vm := vz.NewVirtualMachine(config)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGKILL)

	errCh := make(chan error, 1)

	vm.Start(func(err error) {
		if err != nil {
			errCh <- err
		} else {
			_ = os.WriteFile(m.PidFilePath(), []byte(strconv.Itoa(os.Getpid())), 0600)
		}
	})

	for {
		select {
		case sig := <-signalCh:
			utils.Logger.Info("Receiving a termination signal", sig, "... Bye")
			result, err := vm.RequestStop()
			vm.Release()
			if err != nil {
				utils.Logger.Warn("The machine", m.Name, "was not stop properly: ", err)
			} else if result {
				utils.Logger.Warn("The machine", m.Name, "was stopped successfully")
			} else {
				utils.Logger.Info("The machine", m.Name, "was not stopped")
			}
			os.Remove(m.PidFilePath())
			os.Exit(0)
		case newState := <-vm.StateChangedNotify():
			switch newState {
			case vz.VirtualMachineStateRunning:
				utils.Logger.Info("Machine", m.Name, "change status to running")
				m.ExportMachineSpecification()
			case vz.VirtualMachineStateStarting:
				utils.Logger.Info("Machine", m.Name, "change status to starting on normal sequence")
			case vz.VirtualMachineStateStopped:
				utils.Logger.Info("Machine", m.Name, "change status to stopped")
				return
			default:
				utils.Logger.Info("No action for the state", newState)

			}
		case err := <-errCh:
			utils.Logger.Error("in start:", err)
			return
		}
	}

}

func (m *Machine) launchPrimaryBoot() (vm *vz.VirtualMachine, err error) {

	err = m.Distribution.DownloadDistro()
	if err != nil {
		utils.Logger.Fatal(err)
	}

	if err != nil {
		utils.Logger.Error(err)
	}

	kernelCommandLineArguments := []string{"console=hvc0"}

	bootLoader := vz.NewLinuxBootLoader(
		m.KernelDirectory(),
		vz.WithCommandLine(strings.Join(kernelCommandLineArguments, " ")),
		vz.WithInitrd(m.InitRdDirectory()),
	)

	log.Println("bootLoader:", bootLoader)

	config := vz.NewVirtualMachineConfiguration(
		bootLoader,
		Default_cpu_number,
		default_mem_size,
	)

	input, _ := os.Create(m.inputFilePath())

	serialPortAttachment := vz.NewFileHandleSerialPortAttachment(input, m.Output())
	consoleConfig := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
		consoleConfig,
	})

	// network
	natAttachment := vz.NewNATNetworkDeviceAttachment()
	networkConfig := vz.NewVirtioNetworkDeviceConfiguration(natAttachment)

	mac, err := net.ParseMAC(GenerateAlmostUniqueMac(m.Name))
	networkConfig.SetMacAddress(vz.NewMACAddress(mac))
	config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{
		networkConfig,
	})

	// entropy
	entropyConfig := vz.NewVirtioEntropyDeviceConfiguration()
	config.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{
		entropyConfig,
	})

	diskPath, err := m.RootDirectory()
	if err != nil {
		return
	}
	diskImageAttachment, err := vz.NewDiskImageStorageDeviceAttachment(
		diskPath,
		false,
	)

	if err != nil {
		log.Fatal(err)
	}
	storageDeviceConfig := vz.NewVirtioBlockDeviceConfiguration(diskImageAttachment)
	config.SetStorageDevicesVirtualMachineConfiguration([]vz.StorageDeviceConfiguration{
		storageDeviceConfig,
	})

	// traditional memory balloon device which allows for managing guest memory. (optional)
	config.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{
		vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration(),
	})

	// socket device (optional)
	config.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{
		vz.NewVirtioSocketDeviceConfiguration(),
	})
	validated, err := config.Validate()
	if !validated || err != nil {
		log.Fatal("validation failed", err)
	}

	vm = vz.NewVirtualMachine(config)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGKILL)

	errCh := make(chan error, 1)

	vm.Start(func(err error) {
		if err != nil {
			errCh <- err
		} else {
			_ = os.WriteFile(m.PidFilePath(), []byte(strconv.Itoa(os.Getpid())), 0600)
		}
	})

	for {
		select {
		case signal := <-signalCh:
			utils.Logger.Info("recieved signal", signal)
			_, err = vm.RequestStop()
			if err != nil {
				utils.Logger.Error("Machine", m.Name, " wasn't stopped")
				utils.Logger.Debug(err)
				return
			}
			utils.Logger.Info("Machine", m.Name, " was successfully stopped")
		case newState := <-vm.StateChangedNotify():
			switch newState {
			case vz.VirtualMachineStateRunning:
				utils.Logger.Info("Machine", m.Name, "change status to running, preparing first boot")
				m.prepareFirstBoot()
				m.ExportMachineSpecification()
			case vz.VirtualMachineStateStarting:
				utils.Logger.Info("Machine", m.Name, "change status to starting")
			case vz.VirtualMachineStateStopped:
				utils.Logger.Info("Machine", m.Name, "change status to stopped, will boot on normal sequence")
				return
			default:
				utils.Logger.Info("No action for the state", newState)

			}
		case err := <-errCh:
			utils.Logger.Error("in start:", err)
			return vm, err
		}
	}

	return
}

func (m *Machine) prepareFirstBoot() {
	input := m.Input()
	defer input.Close()

	homedir, _ := os.UserHomeDir()
	sshkey, err := os.ReadFile(fmt.Sprint(homedir, "/.ssh/id_rsa.pub"))
	if err != nil {
		utils.Logger.Fatal("No default ssh key found at /.ssh/id_rsa.pub")
	}
	time.Sleep(5 * time.Second)
	_, err = input.WriteString("mkdir /mnt\n")
	time.Sleep(time.Second)
	input.WriteString("mount /dev/vda /mnt\r")
	time.Sleep(time.Second)
	input.WriteString("cat << EOF > /mnt/etc/cloud/cloud.cfg.d/99_user.cfg\r")
	input.WriteString(fmt.Sprintf(cloudinit, sshkey))
	input.WriteString("\rEOF\r")
	time.Sleep(time.Second)
	input.WriteString("sync\n")
	input.WriteString("poweroff\n")
}

const cloudinit = `
#cloud-config
disable_root: 0

users:
  - name: root
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    lock_passwd: false
    hashed_passwd: $1$SaltSalt$YhgRYajLPrYevs14poKBQ0
    ssh-authorized-keys: 
      - %s
runcmd:
    - [ cp, /usr/bin/true, /usr/sbin/flash-kernel ]
    - [ apt, remove, --purge, irqbalance, -y ]

`
