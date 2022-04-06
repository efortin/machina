package internal

import (
	"errors"
	"fmt"
	"github.com/Code-Hex/vz"
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)
import "C"

type Machine struct {
	Name         string
	Distribution *UbuntuDistribution
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
	inputFile, _ := os.Create(m.inputFilePath())
	return inputFile
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
	err := m.Distribution.copyFileIfNotExist(m.Distribution.InitRdPath(), path)
	if err != nil {
		klog.Exit(err)
	}
	return
}

func (m *Machine) KernelDirectory() (path string) {
	path = fmt.Sprintf("%s/%s", m.BaseDirectory(), "vmlinuz")
	err := m.Distribution.copyFileIfNotExist(m.Distribution.KernelPath(), path)
	if err != nil {
		klog.Exit(err)
	}
	return
}

func (m *Machine) RootDirectory() (path string) {
	path = fmt.Sprintf("%s/%s", m.BaseDirectory(), "root.img")
	err := m.Distribution.copyFileIfNotExist(m.Distribution.ImagePath(), path)
	os.Truncate(path, 8*1024*1024*1024)
	if err != nil {
		klog.Exit(err)
	}
	return
}

func (m *Machine) Launch() (*vz.VirtualMachine, string) {

	kernelCommandLineArguments := []string{"console=hvc0", "root=/dev/vda"}

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

	// console

	//f, err := os.OpenFile("access.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	fmt.Println(m.RootDirectory())
	setRawMode(os.Stdin)

	serialPortAttachment := vz.NewFileHandleSerialPortAttachment(m.Input(), m.Output())
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

	diskImageAttachment, err := vz.NewDiskImageStorageDeviceAttachment(
		m.RootDirectory(),
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

	return vm, mac.String()

}

func (m *Machine) LaunchPrimaryBoot() {

	err := m.Distribution.DownloadDistro()
	if err != nil {
		klog.Fatal(err)
	}

	if err != nil {
		klog.Error(err)
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

	serialPortAttachment := vz.NewFileHandleSerialPortAttachment(m.Input(), m.Output())
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

	diskImageAttachment, err := vz.NewDiskImageStorageDeviceAttachment(
		m.RootDirectory(),
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

	for {
		select {
		case <-signalCh:
			result, err := vm.RequestStop()
			if err != nil {
				log.Println("request stop error:", err)
				return
			}
			log.Println("recieved signal", result)
		case newState := <-vm.StateChangedNotify():
			if newState == vz.VirtualMachineStateRunning {
				log.Println("start VM is running")

				homedir, _ := os.UserHomeDir()
				sshkey, err := os.ReadFile(fmt.Sprint(homedir, "/.ssh/id_rsa.pub"))
				if err != nil {
					klog.Exit("No default ssh key found at /.ssh/id_rsa.pub")
				}

				fmt.Println(fmt.Sprint(homedir, "/.ssh/id_rsa.pub"))
				fmt.Println(string(sshkey))

				input := m.Input()

				time.Sleep(5 * time.Second)
				input.WriteString("mkdir /mnt\n")
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
			if newState == vz.VirtualMachineStateStopped {
				log.Println("stopped successfully")
				return
			}
		case err := <-errCh:
			log.Println("in start:", err)
		}
	}
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
