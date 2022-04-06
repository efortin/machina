package internal

import (
	"errors"
	"fmt"
	"github.com/Code-Hex/vz"
	"github.com/efortin/vz/utils"
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)
import "C"

const (
	default_disk_size = 8 * 1024 * 1024 * 1024
	default_mem_size  = 8 * 1024 * 1024 * 1024
)

type Machine struct {
	Name         string
	Distribution *UbuntuDistribution
}

// IpAddress Return VM ip address if already available
// error if not found
func (m *Machine) IpAddress() (string, error) {
	return GetIPAddressByMACAddress(GenerateAlmostUniqueMac(m.Name))
}

func (m *Machine) outputFilePath() string {
	return fmt.Sprintf("%s/%s", m.BaseDirectory(), "output")
}

func (m *Machine) Output() *os.File {
	outputFile, _ := os.Create(m.outputFilePath())
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

func (m *Machine) BaseDirectory() string {
	basedir := fmt.Sprintf("%s/%s", m.Distribution.baseMachineDirectory(), m.Name)

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

	utils.Logger.Info("resizing disk", default_disk_size, disk.Size())
	if default_disk_size > disk.Size() {
		utils.Logger.Info("resizing disk", disk.Size(), "to", default_disk_size)
		os.Truncate(path, default_disk_size)
	}
	return
}

func (m *Machine) Launch() (machine *vz.VirtualMachine, err error) {

	kernelCommandLineArguments := []string{"console=hvc0", "root=/dev/vda"}

	bootLoader := vz.NewLinuxBootLoader(
		m.KernelDirectory(),
		vz.WithCommandLine(strings.Join(kernelCommandLineArguments, " ")),
		vz.WithInitrd(m.InitRdDirectory()),
	)

	config := vz.NewVirtualMachineConfiguration(
		bootLoader,
		1,
		default_mem_size,
	)

	// console
	input := m.Input()
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
	//networkConfig.SetMacAddress(vz.NewRandomLocallyAdministeredMACAddress())

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
	signal.Notify(signalCh, syscall.SIGTERM)

	errCh := make(chan error, 1)

	vm.Start(func(err error) {
		if err != nil {
			errCh <- err
		}
	})

	return vm, err

}

func (m *Machine) LaunchPrimaryBoot() (vm *vz.VirtualMachine, err error) {

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
		1,
		2*1024*1024*1024,
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
	signal.Notify(signalCh, syscall.SIGTERM)

	errCh := make(chan error, 1)

	vm.Start(func(err error) {
		if err != nil {
			errCh <- err
		}
		input := m.Input()
		defer input.Close()
		m.prepareFirstBoot(input)
	})
	return
}

func (m *Machine) prepareFirstBoot(input *os.File) {
	homedir, _ := os.UserHomeDir()
	sshkey, err := os.ReadFile(fmt.Sprint(homedir, "/.ssh/id_rsa.pub"))
	if err != nil {
		utils.Logger.Fatal("No default ssh key found at /.ssh/id_rsa.pub")
	}
	time.Sleep(5 * time.Second)
	fmt.Println("writing")
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
	time.Sleep(time.Second)

}

func setRawMode(f *os.File) {
	var attr unix.Termios

	// Get settings for terminal
	termios.Tcgetattr(f.Fd(), &attr)

	// Put stdin into raw mode, disabling local echo, input canonicalization,
	// and CR-NL mapping.
	attr.Iflag &^= syscall.ICRNL
	attr.Lflag &^= syscall.ICANON | syscall.ECHO

	// Set minimum characters when reading = 1 char
	attr.Cc[syscall.VMIN] = 1

	// set timeout when reading as non-canonical mode
	attr.Cc[syscall.VTIME] = 0

	// reflects the changed settings
	termios.Tcsetattr(f.Fd(), termios.TCSANOW, &attr)
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
