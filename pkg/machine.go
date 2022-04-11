package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Code-Hex/vz"
	"github.com/efortin/machina/utils"
	"github.com/hpcloud/tail"
	"github.com/mitchellh/go-ps"
	"golang.org/x/crypto/ssh"
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

	Machine_state_running = "running"
	Machine_state_stop    = "stopped"
	Machine_state_error   = "unknown"

	TimeoutStart = 5 * time.Second
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

func (m Machine) Log() {
	t, _ := tail.TailFile(m.OutputLogPath(), tail.Config{Follow: true})
	for line := range t.Lines {
		fmt.Println(line.Text)
	}
}

func InfoFilePath(machineName string) string {
	return fmt.Sprintf("%s/%s", MachineDirectory(machineName), infoFileName)
}

func (d *Machine) InfoFilePath() string {
	return InfoFilePath(d.Name)
}

func (d *Machine) PID() string {
	pid, _, _ := d.findVfkitProcess()
	if pid == nil {
		return utils.Empty
	}
	return strconv.Itoa(pid.Pid())
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
func (d *Machine) findVfkitProcess() (ps.Process, string, error) {
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

	ip, err := d.IpAddress()
	if err == nil {
		client, session, err := connectToHost("root", ip+":22")
		// Could not connect
		if err != nil {
			d.sendSignal()
			return
		}
		session.Run("poweroff")
		session.Close()
		client.Close()
		utils.Logger.Info("Sleeping")
	}
	time.Sleep(10 * time.Second)
	d.sendSignal()

}

func (m *Machine) cleanBeforeExit() {
	os.Remove(m.PidFilePath())
}

func (m *Machine) sendSignal() {
	psProc, machinestate, err := m.findVfkitProcess()
	if machinestate != Machine_state_running {
		return
	}
	utils.Logger.Info("try to kill", psProc.Pid())
	proc, err := os.FindProcess(psProc.Pid())
	if err == nil {
		proc.Signal(syscall.SIGTERM)
	} else {
		utils.Logger.Info("Error during kill", err)
	}
	return
}

// IpAddress Return VM ip address if already available
// error if not found
func (m *Machine) IpAddress() (string, error) {
	return GetIPAddressByMACAddress(GenerateAlmostUniqueMac(m.Name))
}

func (m *Machine) OutputLogPath() string {
	return fmt.Sprintf("%s/%s-%s", TmpDirectory(), m.Name, "output")
}

func (m *Machine) inputLogPath() string {
	return fmt.Sprintf("%s/%s-%s", TmpDirectory(), m.Name, "input")
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
			m.launchPrimaryBoot()
		}
		if err == nil {
			m.launch()
		}
	}
}

func (m *Machine) State() string {
	_, state, _ := m.findVfkitProcess()
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
	serialPortAttachment, err := vz.NewFileSerialPortAttachment(m.OutputLogPath(), true)
	if err != nil {
		utils.Logger.Errorf("Error during serial port attachment (file: %s): %v", m.OutputLogPath(), err)
		os.Exit(1)
	}
	defer input.Close()
	defer output.Close()
	serialPortAttachment := vz.NewFileHandleSerialPortAttachment(input, output)
	consoleConfig := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
		consoleConfig,
	})

	// network
	natAttachment := vz.NewNATNetworkDeviceAttachment()
	networkConfig := vz.NewVirtioNetworkDeviceConfiguration(natAttachment)

	mac, err := net.ParseMAC(GenerateAlmostUniqueMac(m.Name))
	networkConfig.SetMACAddress(vz.NewMACAddress(mac))
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

	err = m.waitForVMState(vm, vz.VirtualMachineStateRunning)
	if err != nil {
		utils.Logger.Error(err)
		m.cleanBeforeExit()
		os.Exit(1)
	}
	m.ExportMachineSpecification()
	m.waitTermination(vm, signalCh)

}

func (m *Machine) waitTermination(vm *vz.VirtualMachine, signalCh chan os.Signal) {
	for {
		select {
		case sig := <-signalCh:
			utils.Logger.Info("Receiving a termination signal", sig, "... Bye")
			result, err := vm.RequestStop()
			if err != nil {
				utils.Logger.Warn("The machine", m.Name, "was not stop properly: ", err)
			} else if result {
				utils.Logger.Warn("The machine", m.Name, "was stopped successfully")
			} else {
				utils.Logger.Info("The machine", m.Name, "was not stopped")
			}
			vm.Release()
			m.cleanBeforeExit()
			os.Exit(0)
		}
	}
}

func (m *Machine) launchPrimaryBoot() {

	err := m.Distribution.DownloadDistro()
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

	input, err := os.OpenFile(m.inputLogPath(), os.O_RDONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		utils.Logger.Error("Error during openning %s file: %v", m.inputLogPath(), err)
		os.Exit(1)
	}
	defer input.Close()
	output, err := os.OpenFile(m.OutputLogPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		utils.Logger.Error("Error during openning %s file: %v", output, err)
		os.Exit(1)
	}
	defer input.Close()
	defer output.Close()
	serialPortAttachment := vz.NewFileHandleSerialPortAttachment(input, output)
	consoleConfig := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
		consoleConfig,
	})

	// network
	natAttachment := vz.NewNATNetworkDeviceAttachment()
	networkConfig := vz.NewVirtioNetworkDeviceConfiguration(natAttachment)

	mac, err := net.ParseMAC(GenerateAlmostUniqueMac(m.Name))
	networkConfig.SetMACAddress(vz.NewMACAddress(mac))
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

	err = m.waitForVMState(vm, vz.VirtualMachineStateRunning)

	if err != nil {
		utils.Logger.Error(err)
		os.Exit(1)
	}
	m.prepareFirstBoot()
	m.ExportMachineSpecification()
	vm.RequestStop()
	err = m.waitForVMState(vm, vz.VirtualMachineStateStopped)
	vm.Release()
	input.Close()

}

func (m *Machine) prepareFirstBoot() {
	input, err := os.OpenFile(m.inputLogPath(), os.O_WRONLY, 0666)
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

func (machine *Machine) waitForVMState(vm *vz.VirtualMachine, state vz.VirtualMachineState) error {
	for {
		select {
		case newState := <-vm.StateChangedNotify():
			if newState == state {
				utils.Logger.Infof("Machine %s reached state %v", machine.Name, state)
				return nil
			}
		case <-time.After(5 * time.Second):
			return fmt.Errorf("Machine %s failed to reached state %v after %v", machine.Name, state, TimeoutStart)
		}
	}
}

func connectToHost(user, host string) (*ssh.Client, *ssh.Session, error) {

	home, _ := os.UserHomeDir()
	pemBytes, err := ioutil.ReadFile(home + "/.ssh/id_rsa")
	if err != nil {
		log.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		log.Fatalf("parse key failed:%v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
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
